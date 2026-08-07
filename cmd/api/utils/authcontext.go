package utils

import "github.com/gin-gonic/gin"

// Claves del contexto de Gin donde AuthMiddleware deja la identidad autenticada.
// Viven en utils (no en app) para que controllers y services puedan leerlas sin
// depender del paquete app, que a su vez depende de ellos.
const (
	AuthUserIDKey    = "auth_user_id"
	AuthSessionIDKey = "auth_session_id"
	AuthRolesKey     = "auth_roles"
)

// GetAuthUserID lee el ID del usuario autenticado seteado por AuthMiddleware.
func GetAuthUserID(c *gin.Context) (int64, bool) {
	v, exists := c.Get(AuthUserIDKey)
	if !exists {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}
