package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/utils"
)

// E2E del broadcast D7: POST /workout-feedback → hook en el controller →
// HubNotifier → Hub → conexión WS suscripta al canal canónico session:{id}.
// La autenticación del upgrade se cubre en los tests del paquete realtime/app:
// acá el handler inyecta el userID fijo para probar el delivery, no el token.

// permissiveSessionAuthorizer autoriza cualquier session:{id>0} canónico.
type permissiveSessionAuthorizer struct{}

func (permissiveSessionAuthorizer) Authorize(channel string, _ int64) (bool, error) {
	raw, ok := strings.CutPrefix(channel, "session:")
	if !ok {
		return false, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || channel != "session:"+strconv.FormatInt(id, 10) {
		return false, nil
	}
	return true, nil
}

// wsBroadcastServer server HTTP con upgrade WS + rutas de feedback reales.
type wsBroadcastServer struct {
	server *httptest.Server
	hub    *realtime.Hub
}

func newFeedbackBroadcastServer(t *testing.T) *wsBroadcastServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hub := realtime.NewHub()
	gateway := realtime.NewGateway(hub, permissiveSessionAuthorizer{}, nil)

	router := gin.New()
	// Auth sintética para la ruta HTTP; el WS ignora el contexto (su authn
	// por token query va cubierta en los tests del paquete realtime/app).
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

	notifier := realtime.NewHubNotifier(hub)
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedbackForSession(42, 7), nil
		},
	}
	ctrl := NewWorkoutFeedbackController(mockSvc, notifier)
	router.POST("/api/v1/workout-feedback", ctrl.Create)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &wsBroadcastServer{server: server, hub: hub}
}

func (s *wsBroadcastServer) postFeedback(t *testing.T) workoutfeedback.MutationResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/api/v1/workout-feedback", createFeedbackRequest(42))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var mutation workoutfeedback.MutationResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&mutation))
	return mutation
}

// wsBroadcastClient replica el cliente de prueba del paquete realtime: dial +
// reader goroutine + frames decodificados en canal propio.
type wsBroadcastClient struct {
	conn   *websocket.Conn
	frames chan setEventFrame
	errs   chan error
}

func (s *wsBroadcastServer) dialSubscriber(t *testing.T, channel string) *wsBroadcastClient {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(s.server.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := &wsBroadcastClient{conn: conn, frames: make(chan setEventFrame, 16), errs: make(chan error, 2)}
	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, raw, err := conn.ReadMessage()
			if err != nil {
				client.errs <- err
				return
			}
			var f setEventFrame
			if uerr := json.Unmarshal(raw, &f); uerr == nil {
				client.frames <- f
			}
		}
	}()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "subscribe", "channel": channel}))
	client.waitSubscribed(t, channel)
	return client
}

func (w *wsBroadcastClient) waitSubscribed(t *testing.T, channel string) {
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

func (w *wsBroadcastClient) waitUpdateSetEvent(t *testing.T) setEventFrame {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout esperando update:set_event")
		case f := <-w.frames:
			if f.Type == realtime.UpdateSetEventType {
				return f
			}
		case err := <-w.errs:
			t.Fatalf("la conexión se cortó esperando update:set_event: %v", err)
		}
	}
}

func (w *wsBroadcastClient) waitSilence(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case f := <-w.frames:
		t.Fatalf("llegó frame inesperado durante el silencio: %+v", f)
	case err := <-w.errs:
		t.Fatalf("la conexión se cortó durante el silencio: %v", err)
	case <-time.After(d):
	}
}

func TestCreateFeedback_BroadcastReachesSubscribedWSClient(t *testing.T) {
	s := newFeedbackBroadcastServer(t)
	target := s.dialSubscriber(t, "session:42")
	// El suscriptor de un canal ajeno debe seguir en silencio (canal targeting).
	bystander := s.dialSubscriber(t, "session:99")

	mutation := s.postFeedback(t)
	require.Equal(t, workoutfeedback.MsgFeedbackCreated, mutation.Message)

	frame := target.waitUpdateSetEvent(t)
	assert.Equal(t, realtime.UpdateSetEventType, frame.Type)
	assert.Equal(t, workoutfeedback.MsgFeedbackCreated, frame.Data.Message)
	assert.Equal(t, int64(7), frame.Data.Data.AthleteUserID)
	assert.Equal(t, int64(42), frame.Data.Data.AssignedSessionID)

	bystander.waitSilence(t, 300*time.Millisecond)
}

func TestCreateFeedback_BroadcastWithNoSubscribersIsNoop(t *testing.T) {
	s := newFeedbackBroadcastServer(t)

	mutation := s.postFeedback(t)
	require.Equal(t, workoutfeedback.MsgFeedbackCreated, mutation.Message)
	assert.Equal(t, 0, s.hub.Count("session:42"))
}
