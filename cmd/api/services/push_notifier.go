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
	// pushSendTimeout es el timeout de cada request de push.
	pushSendTimeout = 10 * time.Second
	// pushFinishTotalBudget acota el fan-out completo del cierre: cada lote
	// tiene su propio timeout, pero el total no supera este techo.
	pushFinishTotalBudget = 30 * time.Second
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
	if pushClient == nil {
		return
	}
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

	sendPushTokenChunks(sendCtx, ctx, pushClient, tokenStrings, title, body, data,
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("notif_type", notifType))
}

// sendPushTokenChunks parte los tokens en lotes de hasta 100 (límite de Expo)
// y los envía best-effort: un lote fallido no frena a los siguientes. logCtx
// conserva el contexto original para los logs (nil cuando no hay request).
func sendPushTokenChunks(sendCtx context.Context, logCtx *gin.Context, pushClient expopushclient.ExpoPushClientInterface, tokens []string, title, body string, data map[string]string, tags ...string) {
	sendPushTokenChunksWithTimeout(sendCtx, logCtx, pushClient, tokens, title, body, data, 0, tags...)
}

// sendPushTokenChunksWithTimeout es la variante del fan-out del cierre: cada
// lote de hasta 100 tokens recibe su propio timeout, siempre acotado por el
// deadline de sendCtx (el presupuesto total). chunkTimeout <= 0 desactiva el
// timeout por lote.
func sendPushTokenChunksWithTimeout(sendCtx context.Context, logCtx *gin.Context, pushClient expopushclient.ExpoPushClientInterface, tokens []string, title, body string, data map[string]string, chunkTimeout time.Duration, tags ...string) {
	if pushClient == nil {
		return
	}
	seen := make(map[string]struct{}, len(tokens))
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		unique = append(unique, token)
	}
	tokens = unique
	for start := 0; start < len(tokens); start += expoPushBatchLimit {
		if err := sendCtx.Err(); err != nil {
			customlogger.Error(logCtx, "push fan-out budget exhausted", err, tags...)
			return
		}
		end := start + expoPushBatchLimit
		if end > len(tokens) {
			end = len(tokens)
		}
		chunkCtx := sendCtx
		cancel := func() {}
		if chunkTimeout > 0 {
			chunkCtx, cancel = context.WithTimeout(sendCtx, chunkTimeout)
		}
		err := pushClient.SendBatch(chunkCtx, tokens[start:end], title, body, data)
		cancel()
		if err != nil {
			customlogger.Error(logCtx, "error sending push notification", err, tags...)
		}
	}
}
