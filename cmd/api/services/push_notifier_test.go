package services

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// TestSendPushToUser_NilContextUsesBackgroundWithTimeout reproduce el bug: un
// *gin.Context nil pasado como context.Context no es un context nil de interfaz
// y SendBatch lo desreferencia al usarlo (Deadline/Err/Do).
func TestSendPushToUser_NilContextUsesBackgroundWithTimeout(t *testing.T) {
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(_ *gin.Context, _ int64) ([]dbs.PushToken, error) {
		return []dbs.PushToken{{Token: "ExponentPushToken[a]"}}, nil
	}}

	var gotCtx context.Context
	var errAtCall error
	pushClient := &mockExpoPushClient{mockSendBatch: func(ctx context.Context, _ []string, _, _ string, _ map[string]string) error {
		gotCtx = ctx
		errAtCall = ctx.Err()
		_, _ = ctx.Deadline()
		return nil
	}}

	require.NotPanics(t, func() {
		sendPushToUser(nil, pushTokenDao, pushClient, 1, "Título", "Cuerpo", "invitation_received", "/invitations")
	})

	require.NotNil(t, gotCtx)
	_, isGinContext := gotCtx.(*gin.Context)
	require.False(t, isGinContext, "SendBatch recibió un *gin.Context tipado como context.Context")

	deadline, ok := gotCtx.Deadline()
	require.True(t, ok)
	remaining := time.Until(deadline)
	assert.Greater(t, remaining, 9*time.Second)
	assert.LessOrEqual(t, remaining, 10*time.Second)
	assert.NoError(t, errAtCall)
}

// TestSendPushToUser_KeepsRequestContext: con un contexto real se conserva el
// que entrega el caller, sin envolverlo en un timeout nuevo.
func TestSendPushToUser_KeepsRequestContext(t *testing.T) {
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(_ *gin.Context, _ int64) ([]dbs.PushToken, error) {
		return []dbs.PushToken{{Token: "ExponentPushToken[a]"}}, nil
	}}

	var gotCtx context.Context
	pushClient := &mockExpoPushClient{mockSendBatch: func(ctx context.Context, _ []string, _, _ string, _ map[string]string) error {
		gotCtx = ctx
		return nil
	}}

	gin.SetMode(gin.ReleaseMode)
	ctx := &gin.Context{Request: nil}

	sendPushToUser(ctx, pushTokenDao, pushClient, 1, "Título", "Cuerpo", "invitation_received", "/invitations")

	require.Same(t, ctx, gotCtx)
}
