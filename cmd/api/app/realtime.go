package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/realtime"
)

// sessionChannelPrefix es el único patrón de canal registrado: session:{id},
// con {id} = ID de session instance. Otros canales (o el prefijo sin número)
// no existen → el authorizer devuelve false y el gateway responde `error`.
const sessionChannelPrefix = "session:"

// newChannelAuthorizer arma el registro de patrones del gateway (D5): el
// patrón session:{id} NO vive en el paquete realtime — acá delega la regla
// dual de acceso a instancia en HasInstanceAccess (Gap 14).
func newChannelAuthorizer(dao daos.SessionInstanceDaoInterface) realtime.ChannelAuthorizer {
	return channelAuthorizerFunc(func(channel string, userID int64) (bool, error) {
		id, ok := parseSessionChannel(channel)
		if !ok {
			return false, nil
		}
		return dao.HasInstanceAccess(nil, id, userID)
	})
}

type channelAuthorizerFunc func(channel string, userID int64) (bool, error)

func (f channelAuthorizerFunc) Authorize(channel string, userID int64) (bool, error) {
	return f(channel, userID)
}

// parseSessionChannel acepta `session:{id}` con id entero > 0.
func parseSessionChannel(channel string) (int64, bool) {
	raw, ok := strings.CutPrefix(channel, sessionChannelPrefix)
	if !ok || raw == "" {
		return 0, false
	}
	var id int64
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// wsUpgrade es el handler gin de la ruta pública GET /api/v1/ws (D3): valida
// el token del query param ANTES del upgrade — falla → 401 JSON con el mismo
// contrato de error que AuthMiddleware — y luego delega upgrade + pumps en
// realtime.Gateway.
func wsUpgrade(gateway *realtime.Gateway) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       "unauthorized",
				Message:    "falta el query param token",
			})
			return
		}

		claims, err := authorizeAccessToken(token)
		if err != nil {
			code, message := "unauthorized", "token inválido"
			if errors.Is(err, jwt.ErrTokenExpired) {
				code, message = "token_expired", "el access token expiró"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       code,
				Message:    message,
			})
			return
		}

		conn, err := gateway.Upgrade(c.Writer, c.Request)
		if err != nil {
			// El Upgrader ya escribió la respuesta HTTP de error (p.ej. Origin prohibido).
			customlogger.Info(c, "upgrade websocket rechazado", customlogger.Tag("error", err.Error()))
			return
		}
		gateway.Serve(conn, claims.UserID)
	}
}
