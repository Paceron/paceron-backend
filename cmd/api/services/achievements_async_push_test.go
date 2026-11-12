package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// blockingPushTokenDAO frena la lectura de tokens hasta que el test libere:
// así se garantiza que la cancelación del request ocurra antes del envío.
type blockingPushTokenDAO struct {
	started chan struct{}
	release chan struct{}
	logCtx  chan *gin.Context
	tokens  []dbs.PushToken
}

func (f *blockingPushTokenDAO) Upsert(_ *gin.Context, _ int64, _, _ string) error {
	return nil
}

func (f *blockingPushTokenDAO) FindByUserID(ctx *gin.Context, _ int64) ([]dbs.PushToken, error) {
	f.logCtx <- ctx
	f.started <- struct{}{}
	<-f.release
	return f.tokens, nil
}

func (f *blockingPushTokenDAO) FindByUserIDs(_ *gin.Context, _ []int64) ([]dbs.PushToken, error) {
	return f.tokens, nil
}

// capturedSend copia el estado del contexto en el momento exacto del envío:
// después SendBatch retorna y el defer del timeout lo cancela.
type capturedSend struct {
	err         error
	deadline    time.Time
	hasDeadline bool
}

// contextCapturingPushClient guarda el contexto HTTP que recibe SendBatch.
type contextCapturingPushClient struct {
	sent chan capturedSend
}

func (c *contextCapturingPushClient) Send(_ context.Context, _, _, _ string, _ map[string]string) error {
	return nil
}

func (c *contextCapturingPushClient) SendBatch(ctx context.Context, _ []string, _, _ string, _ map[string]string) error {
	deadline, ok := ctx.Deadline()
	c.sent <- capturedSend{err: ctx.Err(), deadline: deadline, hasDeadline: ok}
	return nil
}

// TestAchievementsService_AsyncPushContextSurvivesRequestCancel es la
// regresión del review final: los hooks async usan un *gin.Context copiado,
// cuyo Request.Context se cancela al responder. El push debe salir con un
// contexto independiente con timeout, no con el del request. OnEventBatch y
// OnEvent comparten evaluateValues, el punto donde vive el envío.
func TestAchievementsService_AsyncPushContextSurvivesRequestCancel(t *testing.T) {
	tests := []struct {
		name     string
		dispatch func(*achievementsService, *gin.Context)
	}{
		{
			name: "OnEventBatch",
			dispatch: func(svc *achievementsService, ctx *gin.Context) {
				svc.OnEventBatch(ctx, []int64{101}, AchievementTriggerAttendance)
			},
		},
		{
			name: "OnEvent",
			dispatch: func(svc *achievementsService, ctx *gin.Context) {
				svc.OnEvent(ctx, 101, AchievementTriggerAttendance)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertAsyncPushContextSurvivesRequestCancel(t, tt.dispatch)
		})
	}
}

func assertAsyncPushContextSurvivesRequestCancel(t *testing.T, dispatch func(*achievementsService, *gin.Context)) {
	t.Helper()
	dao := newPerUserAttendanceDao(map[int64]int64{101: 1})
	tokens := &blockingPushTokenDAO{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		logCtx:  make(chan *gin.Context, 1),
		tokens:  []dbs.PushToken{{Token: "ExponentPushToken[a]"}},
	}
	client := &contextCapturingPushClient{sent: make(chan capturedSend, 1)}
	svc := NewAchievementsService(dao, &mockGamificationDao{}, &fakeStreakView{}, tokens, client).(*achievementsService)

	ginCtx, engine := gin.CreateTestContext(httptest.NewRecorder())
	// Habilita el fallback al Request.Context para que la cancelación del
	// request sea observable a través del *gin.Context, como en el finding.
	engine.ContextWithFallback = true
	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/attendance", nil).WithContext(reqCtx)
	ginCtx.Set("x-request-id", "rid-async-push")

	dispatch(svc, ginCtx)

	select {
	case <-tokens.started:
	case <-time.After(2 * time.Second):
		t.Fatal("el hook async no llegó a leer tokens")
	}

	cancel()
	close(tokens.release)

	var send capturedSend
	select {
	case send = <-client.sent:
	case <-time.After(2 * time.Second):
		t.Fatal("SendBatch no recibió el push")
	}

	require.NoError(t, send.err, "el contexto HTTP del push no debe heredar la cancelación del request")
	require.True(t, send.hasDeadline, "el push async debe usar un timeout acotado")
	assert.Greater(t, time.Until(send.deadline), time.Duration(0))
	assert.LessOrEqual(t, time.Until(send.deadline), pushSendTimeout)

	logCtx := <-tokens.logCtx
	require.ErrorIs(t, logCtx.Request.Context().Err(), context.Canceled,
		"el request ya estaba cancelado cuando salió el push (repro del bug)")
	requestID, ok := logCtx.Get("x-request-id")
	require.True(t, ok, "el contexto de logging debe conservar la key del request")
	assert.Equal(t, "rid-async-push", requestID)
}
