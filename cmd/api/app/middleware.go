package app

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/utils"
)

const (
	_XrequestID  = "X-Request-Id"
	_Flow        = "Flow"
	_StringEmpty = ""
)

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

func CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	var origins []string
	if allowedOrigins != "" {
		for _, o := range strings.Split(allowedOrigins, ",") {
			origins = append(origins, strings.TrimSpace(o))
		}
	} else {
		origins = []string{
			"http://localhost:8081",
			"http://localhost:5173",
			"http://localhost:3000",
			"https://paceron-frontend.vercel.app",
			"https://paceron-frontend-git-develop-paceron.vercel.app",
		}
	}

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

		tokenString := strings.TrimPrefix(header, prefix)
		claims, err := utils.ParseAccessToken(tokenString)
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

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       "unauthorized",
				Message:    "token inválido",
			})
			return
		}

		c.Set(utils.AuthUserIDKey, userID)
		c.Set(utils.AuthSessionIDKey, claims.SessionID)
		c.Set(utils.AuthRolesKey, claims.Roles)
		c.Next()
	}
}
