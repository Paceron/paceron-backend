package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// E2E del evento D10 (Gap 26): POST /runner del owner → hook de apertura (D7)
// → emitSessionState → HubNotifier → Hub → conexión WS suscripta a
// session:{id}. PATCH finished del owner → cierre. Mismo harness que
// workout_feedback_broadcast_ws_test.go; la auth del upgrade WS va cubierta
// en los tests del paquete realtime/app.

// sessionStateFrame es el frame update:session_state con el objeto data D10.
type sessionStateFrame struct {
	Type string `json:"type"`
	Data struct {
		PresencialOpen bool       `json:"presencial_open"`
		OpenedAt       *time.Time `json:"opened_at"`
		ClosedAt       *time.Time `json:"closed_at"`
	} `json:"data"`
}

type rawFrame struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func presencialDayFixture(opened, closed *time.Time) *dbs.GroupCalendarDay {
	return &dbs.GroupCalendarDay{
		ID:                 9,
		GroupID:            3,
		Kind:               "training",
		IsPresencial:       true,
		PresencialOpenedAt: opened,
		PresencialClosedAt: closed,
	}
}

type runnerBroadcastServer struct {
	server *httptest.Server
	hub    *realtime.Hub
}

func newRunnerBroadcastServer(t *testing.T, hub *realtime.Hub, presencial services.PresencialSessionServiceInterface, notifier realtime.Notifier) *runnerBroadcastServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gateway := realtime.NewGateway(hub, permissiveSessionAuthorizer{}, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, int64(7))
		c.Next()
	})
	router.GET("/api/v1/ws", func(c *gin.Context) {
		conn, err := gateway.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		gateway.Serve(conn, 7)
	})

	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
			return fixtureRunnerSessionResponse(), true, nil
		},
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "finished"
			now := time.Now()
			rs.EndDate = &now
			return rs, nil
		},
	}
	ctrl := NewRunnerSessionController(mockSvc, presencial, notifier)
	router.POST("/api/v1/session-instances/:id/runner", ctrl.Create)
	router.PATCH("/api/v1/session-instances/:id/runner", ctrl.Finish)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &runnerBroadcastServer{server: server, hub: hub}
}

func (s *runnerBroadcastServer) postRunner(t *testing.T) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/api/v1/session-instances/42/runner", strings.NewReader(`{"start_date":"2026-09-24T09:00:00Z"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func (s *runnerBroadcastServer) patchRunnerFinished(t *testing.T) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, s.server.URL+"/api/v1/session-instances/42/runner", strings.NewReader(`{"status":"finished"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// runnerBroadcastClient replica el cliente del harness de feedback: dial +
// reader goroutine + frames como raw maps (los tipos de frame difieren).
type runnerBroadcastClient struct {
	conn   *websocket.Conn
	frames chan rawFrame
	errs   chan error
}

func (s *runnerBroadcastServer) dialSubscriber(t *testing.T, channel string) *runnerBroadcastClient {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(s.server.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := &runnerBroadcastClient{conn: conn, frames: make(chan rawFrame, 16), errs: make(chan error, 2)}
	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, raw, err := conn.ReadMessage()
			if err != nil {
				client.errs <- err
				return
			}
			var f rawFrame
			if uerr := json.Unmarshal(raw, &f); uerr == nil {
				client.frames <- f
			}
		}
	}()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "subscribe", "channel": channel}))
	client.waitSubscribed(t)
	return client
}

func (w *runnerBroadcastClient) waitSubscribed(t *testing.T) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout esperando subscribed")
		case f := <-w.frames:
			if f.Type == "subscribed" {
				return
			}
		case err := <-w.errs:
			t.Fatalf("la conexión se cortó esperando subscribed: %v", err)
		}
	}
}

func (w *runnerBroadcastClient) waitSessionState(t *testing.T) sessionStateFrame {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout esperando update:session_state")
		case f := <-w.frames:
			if f.Type == realtime.UpdateSessionStateEventType {
				var frame sessionStateFrame
				frame.Type = f.Type
				require.NoError(t, json.Unmarshal(f.Data, &frame.Data))
				return frame
			}
		case err := <-w.errs:
			t.Fatalf("la conexión se cortó esperando update:session_state: %v", err)
		}
	}
}

func (w *runnerBroadcastClient) waitSilence(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case f := <-w.frames:
		t.Fatalf("llegó frame inesperado durante el silencio: %+v", f)
	case err := <-w.errs:
		t.Fatalf("la conexión se cortó durante el silencio: %v", err)
	case <-time.After(d):
	}
}

func TestRunnerSession_Open_BroadcastsSessionStateToWS(t *testing.T) {
	opened := time.Now().UTC()
	presencial := &mockPresencialGateway{onCreatedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
		return presencialDayFixture(&opened, nil), true, nil
	}}
	// Hub real: el frame debe cruzar hub→WS (no solo el stub del channel).
	hub := realtime.NewHub()
	s := newRunnerBroadcastServer(t, hub, presencial, realtime.NewHubNotifier(hub))
	target := s.dialSubscriber(t, "session:42")

	require.Equal(t, http.StatusCreated, s.postRunner(t))

	frame := target.waitSessionState(t)
	assert.Equal(t, realtime.UpdateSessionStateEventType, frame.Type)
	assert.True(t, frame.Data.PresencialOpen)
	require.NotNil(t, frame.Data.OpenedAt)
	assert.Nil(t, frame.Data.ClosedAt)
}

func TestRunnerSession_Close_BroadcastsSessionStateToWS(t *testing.T) {
	opened := time.Now().UTC().Add(-2 * time.Hour)
	closed := time.Now().UTC()
	presencial := &mockPresencialGateway{onFinishedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
		return presencialDayFixture(&opened, &closed), true, nil
	}}
	hub := realtime.NewHub()
	s := newRunnerBroadcastServer(t, hub, presencial, realtime.NewHubNotifier(hub))
	target := s.dialSubscriber(t, "session:42")

	require.Equal(t, http.StatusOK, s.patchRunnerFinished(t))

	frame := target.waitSessionState(t)
	assert.Equal(t, realtime.UpdateSessionStateEventType, frame.Type)
	assert.False(t, frame.Data.PresencialOpen)
	require.NotNil(t, frame.Data.OpenedAt)
	require.NotNil(t, frame.Data.ClosedAt)
}

func TestRunnerSession_NilNotifier_NoPanic(t *testing.T) {
	opened := time.Now().UTC()
	presencial := &mockPresencialGateway{onCreatedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
		return presencialDayFixture(&opened, nil), true, nil
	}}
	s := newRunnerBroadcastServer(t, realtime.NewHub(), presencial, nil)

	require.Equal(t, http.StatusCreated, s.postRunner(t))
}

func TestRunnerSession_NonOwner_DoesNotEmit(t *testing.T) {
	// Corredor no-owner: el hook resuelve sin mutación → sin frame para los
	// suscriptos del canal.
	presencial := &mockPresencialGateway{onCreatedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
		return nil, false, nil
	}}
	hub := realtime.NewHub()
	s := newRunnerBroadcastServer(t, hub, presencial, realtime.NewHubNotifier(hub))
	target := s.dialSubscriber(t, "session:42")

	require.Equal(t, http.StatusCreated, s.postRunner(t))
	target.waitSilence(t, 300*time.Millisecond)
}
