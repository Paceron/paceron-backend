package services

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
)

const (
	pushSendTimeout = 10 * time.Second
	// expoPushBatchLimit es el máximo de tokens por request que acepta Expo.
	expoPushBatchLimit = 100
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

	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}
	if len(tokenStrings) == 0 {
		return
	}

	sendCtx := context.Background()
	var cancel context.CancelFunc
	if ctx != nil {
		sendCtx = ctx
	} else {
		sendCtx, cancel = context.WithTimeout(sendCtx, pushSendTimeout)
		defer cancel()
	}

	sendPushTokenChunks(sendCtx, pushClient, tokenStrings, title, body, data,
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("notif_type", notifType))
}

// sendPushTokenChunks parte los tokens en lotes de hasta 100 (límite de Expo)
// y los envía best-effort: un lote fallido no frena a los siguientes.
func sendPushTokenChunks(ctx context.Context, pushClient expopushclient.ExpoPushClientInterface, tokens []string, title, body string, data map[string]string, tags ...string) {
	for start := 0; start < len(tokens); start += expoPushBatchLimit {
		end := start + expoPushBatchLimit
		if end > len(tokens) {
			end = len(tokens)
		}
		if err := pushClient.SendBatch(ctx, tokens[start:end], title, body, data); err != nil {
			customlogger.Error(nil, "error sending push notification", err, tags...)
		}
	}
}
