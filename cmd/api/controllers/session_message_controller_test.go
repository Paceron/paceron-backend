package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/sessionmessage"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
)

type mockSessionMessageService struct {
	createFn func(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error)
	listFn   func(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error)
}

func (m *mockSessionMessageService) Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, authUserID, sessionInstanceID, req)
	}
	return nil, nil
}

func (m *mockSessionMessageService) List(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error) {
	if m.listFn != nil {
		return m.listFn(ctx, authUserID, sessionInstanceID, sinceID)
	}
	return nil, nil
}

func messageServiceRouter(svc services.SessionMessageServiceInterface, notifier realtime.Notifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		setAuthUserID(c, 7)
		c.Next()
	})
	r.POST("/session-instances/:id/messages", NewSessionMessageController(svc, notifier).Create)
	r.GET("/session-instances/:id/messages", NewSessionMessageController(svc, notifier).List)
	return r
}

func messageResponseFor() *sessionmessage.SessionMessageResponse {
	return &sessionmessage.SessionMessageResponse{
		ID:                9,
		SessionInstanceID: 88,
		SenderUserID:      7,
		SenderRole:        "trainer",
		Type:              sessionmessage.TypeMessageAviso,
		RecipientMode:     sessionmessage.RecipientModeMultiple,
		RecipientUserIDs:  []int64{12, 13},
		Body:              "Agrupense en el puente",
		ReplyToMessageID:  nil,
		CreatedAt:         time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC),
	}
}

func TestSessionMessageController_Create_SuccessShape(t *testing.T) {
	svc := &mockSessionMessageService{createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
		require.Equal(t, int64(7), authUserID)
		require.Equal(t, int64(88), sessionInstanceID)
		require.Equal(t, "aviso", req.Type)
		return messageResponseFor(), nil
	}}
	r := messageServiceRouter(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`{"type":"aviso","recipient_mode":"multiple","recipient_user_ids":[12,13],"body":"Agrupense en el puente"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{
		"id": 9, "session_instance_id": 88, "sender_user_id": 7, "sender_role": "trainer",
		"type": "aviso", "recipient_mode": "multiple", "recipient_user_ids": [12, 13],
		"body": "Agrupense en el puente", "reply_to_message_id": null,
		"created_at": "2026-10-09T15:00:00Z"
	}`, rec.Body.String())
}

func TestSessionMessageController_Create_EmitsBroadcastFrame(t *testing.T) {
	svc := &mockSessionMessageService{createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
		return messageResponseFor(), nil
	}}
	notifier := &recordingNotifier{}
	r := messageServiceRouter(svc, notifier)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`{"type":"aviso","recipient_mode":"all","body":"Agrupense"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, notifier.channels, 1, "el POST exitoso emite exactamente un aviso")
	assert.Equal(t, "session:88", notifier.channels[0])
	var frame struct {
		Type    string          `json:"type"`
		Channel string          `json:"channel"`
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(notifier.payloads[0], &frame))
	assert.Equal(t, realtime.ControlMessageCreatedEventType, frame.Type)
	assert.Equal(t, "session:88", frame.Channel)
	var payload map[string]float64
	require.NoError(t, json.Unmarshal(frame.Payload, &payload))
	assert.Equal(t, 9.0, payload["sessionMessageId"])
}

func TestSessionMessageController_Create_NilNotifierDoesNotPanic(t *testing.T) {
	svc := &mockSessionMessageService{createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
		return messageResponseFor(), nil
	}}
	r := messageServiceRouter(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`{"type":"aviso","recipient_mode":"all","body":"Agrupense"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestSessionMessageController_Create_ErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		toStatus int
		code     string
	}{
		{"invalido 400", services.ErrSessionMessageInvalid, http.StatusBadRequest, "Bad request"},
		{"sin acceso 403", services.ErrSessionMessageForbidden, http.StatusForbidden, "Forbidden"},
		{"instancia inexistente 404", services.ErrCalendarInstanceNotFound, http.StatusNotFound, "Not Found"},
	}
	for _, tc := range cases {
		svc := &mockSessionMessageService{createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
			return nil, tc.err
		}}
		r := messageServiceRouter(svc, nil)
		req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`{"type":"aviso","recipient_mode":"all","body":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, tc.toStatus, rec.Code, tc.name)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, tc.code, body["code"], tc.name)
	}
}

func TestSessionMessageController_Create_InstanceIDInvalid(t *testing.T) {
	r := messageServiceRouter(&mockSessionMessageService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/abc/messages", strings.NewReader(`{"type":"aviso","recipient_mode":"all","body":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSessionMessageController_Create_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/session-instances/:id/messages", NewSessionMessageController(&mockSessionMessageService{}, nil).Create)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionMessageController_Create_BadJSON(t *testing.T) {
	r := messageServiceRouter(&mockSessionMessageService{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/session-instances/88/messages", strings.NewReader(`no-json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSessionMessageController_List_ShapeAndCursor(t *testing.T) {
	var gotSince int64
	svc := &mockSessionMessageService{listFn: func(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error) {
		gotSince = sinceID
		return &sessionmessage.MessagesListResponse{Messages: []sessionmessage.SessionMessageResponse{
			{
				ID:                9,
				SessionInstanceID: 88,
				SenderUserID:      7,
				SenderRole:        "trainer",
				Type:              "info",
				RecipientMode:     "all",
				RecipientUserIDs:  []int64{},
				Body:              "nos vemos en el parque",
				ReplyToMessageID:  nil,
				CreatedAt:         time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC),
			},
		}}, nil
	}}
	r := messageServiceRouter(svc, nil)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/88/messages?since=5", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(5), gotSince)
	assert.JSONEq(t, `{"messages":[{
		"id": 9, "session_instance_id": 88, "sender_user_id": 7, "sender_role": "trainer",
		"type": "info", "recipient_mode": "all", "recipient_user_ids": [],
		"body": "nos vemos en el parque", "reply_to_message_id": null,
		"created_at": "2026-10-09T15:00:00Z"
	}]}`, rec.Body.String())
}

func TestSessionMessageController_List_SinceDefaultsToZero(t *testing.T) {
	var gotSince int64
	svc := &mockSessionMessageService{listFn: func(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error) {
		gotSince = sinceID
		return &sessionmessage.MessagesListResponse{Messages: []sessionmessage.SessionMessageResponse{}}, nil
	}}
	r := messageServiceRouter(svc, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/session-instances/88/messages", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(0), gotSince)
}

func TestSessionMessageController_List_InvalidSince(t *testing.T) {
	cases := map[string]int{"abc": http.StatusBadRequest, "-1": http.StatusBadRequest}
	for raw, want := range cases {
		r := messageServiceRouter(&mockSessionMessageService{}, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/session-instances/88/messages?since="+raw, nil))
		assert.Equal(t, want, rec.Code, "since=%s", raw)
	}
}

func TestSessionMessageController_List_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"instancia inexistente", services.ErrCalendarInstanceNotFound, http.StatusNotFound},
		{"sin acceso", services.ErrSessionMessageForbidden, http.StatusForbidden},
		{"invalido", services.ErrSessionMessageInvalid, http.StatusBadRequest},
	}
	for _, tc := range cases {
		svc := &mockSessionMessageService{listFn: func(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error) {
			return nil, tc.err
		}}
		r := messageServiceRouter(svc, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/session-instances/88/messages", nil))
		assert.Equal(t, tc.want, rec.Code, tc.name)
	}
}

func TestSessionMessageController_List_InstanceIDInvalid(t *testing.T) {
	r := messageServiceRouter(&mockSessionMessageService{}, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/session-instances/abc/messages", nil))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
