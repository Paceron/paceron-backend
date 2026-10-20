package app

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/utils"
)

const (
	_XrequestID  = "X-Request-Id"
	_Flow        = "Flow"
	_StringEmpty = ""
)

// maxRequestBodyBytes limita globalmente el tamaño de cualquier body (S9).
// 10 MB queda por encima del tope propio de fotos/ícono (5 MB + overhead),
// así que su MaxBytesReader interno sigue marcando el corte antes.
const maxRequestBodyBytes = 10 << 20

func MaxBodySize() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		}
		c.Next()
	}
}

// Acota el abuso de las rutas públicas de usuario (S10): 30 req/min por IP,
// burst 10. Hardcodeados a propósito (sin env).
const (
	ipRateBurst  = 10
	ipIdleTTL    = time.Hour
	ipPruneEvery = 256
)

// ipRateLimit es var porque rate.Every no es constante: 30 req/min.
var ipRateLimit = rate.Every(time.Minute / 30)

type ipRateEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter acumula un limiter por IP en memoria. Single instance (Render)
// lo hace suficiente; un restart de la app reinicia los contadores, aceptado.
type ipRateLimiter struct {
	mu         sync.Mutex
	entries    map[string]*ipRateEntry
	sincePrune int
}

func newIPRateLimiter() *ipRateLimiter {
	return &ipRateLimiter{entries: make(map[string]*ipRateEntry)}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok {
		entry = &ipRateEntry{limiter: rate.NewLimiter(ipRateLimit, ipRateBurst)}
		l.entries[ip] = entry
	}
	entry.lastSeen = time.Now()

	l.sincePrune++
	if l.sincePrune >= ipPruneEvery {
		l.sincePrune = 0
		for key, e := range l.entries {
			if time.Since(e.lastSeen) > ipIdleTTL {
				delete(l.entries, key)
			}
		}
	}
	return entry.limiter.Allow()
}

var publicIPRateLimiter = newIPRateLimiter()

func IPRateLimiter() gin.HandlerFunc {
	return ipRateLimiterHandler(publicIPRateLimiter)
}

func ipRateLimiterHandler(limiter *ipRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, apierror.APIError{
				StatusCode: http.StatusTooManyRequests,
				Code:       "too_many_requests",
				Message:    "demasiadas solicitudes, intentá de nuevo más tarde",
			})
			return
		}
		c.Next()
	}
}

func SetRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil {
			rqID := c.Request.Header.Get(_XrequestID)
			if rqID == _StringEmpty {
				uuid4, _ := uuid.NewV4()
				rqID = uuid4.String()
			}
			c.Set(_XrequestID, rqID)
			c.Writer.Header().Set(_XrequestID, rqID)
			c.Next()
		}
	}
}

func SetFlow() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil {
			flow := c.Request.URL.Path
			c.Set(_Flow, flow)
			c.Writer.Header().Set(_Flow, flow)
			c.Next()
		}
	}
}

// allowedOrigins devuelve la lista configurada de orígenes permitidos.
// CORSMiddleware y el upgrade del gateway WebSocket la comparten: al agregar
// un dominio nuevo no hay que actualizar dos lecturas del env.
func allowedOrigins() []string {
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		return []string{
			"http://localhost:8081",
			"http://localhost:8082",
			"http://localhost:8083",
			"http://localhost:5173",
			"http://localhost:3000",
			"https://paceron-frontend.vercel.app",
			"https://paceron-frontend-git-develop-paceron.vercel.app",
		}
	}

	origins := make([]string, 0, strings.Count(allowedOrigins, ",")+1)
	for _, o := range strings.Split(allowedOrigins, ",") {
		origins = append(origins, strings.TrimSpace(o))
	}
	return origins
}

func CORSMiddleware() gin.HandlerFunc {
	origins := allowedOrigins()
	originMap := make(map[string]bool, len(origins))
	for _, o := range origins {
		originMap[o] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if originMap[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-Id, X-Current-Password")
			c.Header("Access-Control-Max-Age", "86400")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// authorizeAccessToken valida un access token crudo (sin el prefijo Bearer) y
// devuelve el userID de su subject. Compartido por AuthMiddleware (header) y
// el upgrade del gateway WS (query param): mismo parseo/errores.
func authorizeAccessToken(tokenString string) (*utils.AccessTokenClaims, error) {
	claims, err := utils.ParseAccessToken(tokenString)
	if err != nil {
		return nil, err
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, errors.New("el subject del token no es un user id válido")
	}
	claims.UserID = userID
	return claims, nil
}

// AuthMiddleware valida el access token del header Authorization y deja la identidad
// del usuario disponible en el contexto (auth_user_id, auth_session_id, auth_roles)
// para que los controllers/services la usen en vez de confiar en un user_id que mande
// el cliente. No emite ni renueva tokens — esa responsabilidad es del frontend vía
// POST /api/v1/auth/refresh.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Request.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       "unauthorized",
				Message:    "falta el header Authorization",
			})
			return
		}

		claims, err := authorizeAccessToken(strings.TrimPrefix(header, prefix))
		if err != nil {
			code := "unauthorized"
			message := "token inválido"
			if errors.Is(err, jwt.ErrTokenExpired) {
				code = "token_expired"
				message = "el access token expiró"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       code,
				Message:    message,
			})
			return
		}

		c.Set(utils.AuthUserIDKey, claims.UserID)
		c.Set(utils.AuthSessionIDKey, claims.SessionID)
		c.Set(utils.AuthRolesKey, claims.Roles)
		c.Next()
	}
}
