package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
)

// sendPushToUser dispara una notificación push a todos los dispositivos registrados
// de un usuario. Es una función de paquete (no un service) a propósito: varios
// services (invitation, team_user, user) la necesitan, y .agentics/CONVENTIONS.md
// prohíbe que un service importe a otro — mismo criterio ya usado en la sesión para
// isEntrenadorOfTeam.
//
// Best-effort: un fallo de push (token inválido, Expo caído, sin dispositivos
// registrados) se loguea y nunca bloquea el flujo principal — mismo criterio que
// el envío de mail.
func sendPushToUser(
	ctx *gin.Context,
	pushTokenDao daos.PushTokenDaoInterface,
	pushClient expopushclient.ExpoPushClientInterface,
	userID int64,
	title, body, notifType, route string,
) {
	tokens, err := pushTokenDao.FindByUserID(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding push tokens for notification", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)))
		return
	}

	data := map[string]string{"type": notifType}
	if route != "" {
		data["route"] = route
	}

	for _, token := range tokens {
		if err := pushClient.Send(ctx, token.Token, title, body, data); err != nil {
			customlogger.Error(ctx, "error sending push notification", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.Tag("notif_type", notifType))
		}
	}
}
