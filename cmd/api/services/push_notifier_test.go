package services

import (
	"context"
	"fmt"
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

// TestSendPushToUser_ChunksAt100: reutiliza el chunking común y nunca manda
// más de 100 tokens por request (límite de Expo).
func TestSendPushToUser_ChunksAt100(t *testing.T) {
	tokens := make([]dbs.PushToken, 250)
	for i := range tokens {
		tokens[i] = dbs.PushToken{Token: "ExponentPushToken[" + fmt.Sprintf("%d", i) + "]"}
	}
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(_ *gin.Context, _ int64) ([]dbs.PushToken, error) {
		return tokens, nil
	}}

	var chunkSizes []int
	pushClient := &mockExpoPushClient{mockSendBatch: func(_ context.Context, got []string, _, _ string, _ map[string]string) error {
		chunkSizes = append(chunkSizes, len(got))
		return nil
	}}

	sendPushToUser(nil, pushTokenDao, pushClient, 1, "Título", "Cuerpo", "invitation_received", "/invitations")

	assert.Equal(t, []int{100, 100, 50}, chunkSizes)
}

// TestSendPushToUser_NilClient_NoPanic: sin cliente de push configurado el
// envío es un no-op (no debe desreferenciar nil).
func TestSendPushToUser_NilClient_NoPanic(t *testing.T) {
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(_ *gin.Context, _ int64) ([]dbs.PushToken, error) {
		return []dbs.PushToken{{Token: "ExponentPushToken[a]"}}, nil
	}}

	require.NotPanics(t, func() {
		sendPushToUser(nil, pushTokenDao, nil, 1, "Título", "Cuerpo", "invitation_received", "/invitations")
	})
}

// TestSendPushTokenChunks_FiltersEmptyAndDuplicates: el chunker común descarta
// tokens vacíos y repetidos antes de partir en lotes.
func TestSendPushTokenChunks_FiltersEmptyAndDuplicates(t *testing.T) {
	var chunks [][]string
	pushClient := &mockExpoPushClient{mockSendBatch: func(_ context.Context, got []string, _, _ string, _ map[string]string) error {
		chunks = append(chunks, append([]string(nil), got...))
		return nil
	}}

	require.NotPanics(t, func() {
		sendPushTokenChunks(context.Background(), nil, nil, []string{"a"}, "T", "B", nil)
	})

	sendPushTokenChunks(context.Background(), nil, pushClient,
		[]string{"a", "", "b", "a", "c", ""}, "T", "B", nil)

	require.Len(t, chunks, 1)
	assert.Equal(t, []string{"a", "b", "c"}, chunks[0])
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

// TestSendPushTokenChunksWithTimeout_PerChunkTimeout: cada lote de 100 tokens
// arranca con su propio deadline, no con el del presupuesto total.
func TestSendPushTokenChunksWithTimeout_PerChunkTimeout(t *testing.T) {
	tokens := make([]string, 250)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("ExponentPushToken[%d]", i)
	}
	var deadlines []time.Time
	pushClient := &mockExpoPushClient{mockSendBatch: func(ctx context.Context, _ []string, _, _ string, _ map[string]string) error {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		deadlines = append(deadlines, deadline)
		return nil
	}}

	parent, cancel := context.WithTimeout(context.Background(), pushFinishTotalBudget)
	defer cancel()
	sendPushTokenChunksWithTimeout(parent, nil, pushClient, tokens, "T", "B", nil, pushSendTimeout)

	require.Len(t, deadlines, 3)
	for _, deadline := range deadlines {
		remaining := time.Until(deadline)
		assert.Greater(t, remaining, 9*time.Second)
		assert.LessOrEqual(t, remaining, 10*time.Second)
	}
}

// TestSendPushTokenChunksWithTimeout_BoundedByParent: el presupuesto total
// corta el fan-out en el primer lote, sin pagar retries de los restantes.
func TestSendPushTokenChunksWithTimeout_BoundedByParent(t *testing.T) {
	tokens := make([]string, 250)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("ExponentPushToken[%d]", i)
	}
	var firstDeadline time.Time
	calls := 0
	pushClient := &mockExpoPushClient{mockSendBatch: func(ctx context.Context, _ []string, _, _ string, _ map[string]string) error {
		calls++
		if calls == 1 {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			firstDeadline = deadline
		}
		<-ctx.Done()
		return ctx.Err()
	}}

	parent, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	parentDeadline, ok := parent.Deadline()
	require.True(t, ok)
	start := time.Now()
	sendPushTokenChunksWithTimeout(parent, nil, pushClient, tokens, "T", "B", nil, time.Second)

	assert.Less(t, time.Since(start), 500*time.Millisecond)
	assert.Equal(t, 1, calls, "el fan-out corta al agotarse el presupuesto")
	assert.False(t, firstDeadline.IsZero())
	assert.False(t, firstDeadline.After(parentDeadline), "el deadline del lote respeta el presupuesto total")
	assert.False(t, firstDeadline.After(start.Add(time.Second)), "el deadline del lote no supera el timeout por chunk")
}
