package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSetEventPayloadShape: el data del frame replica el body HTTP del
// endpoint que origina el evento (D4) para reutilizar el normalizador frontend.
func TestUpdateSetEventPayloadShape(t *testing.T) {
	raw := MarshalUpdateSetEvent("session:7", Remote{Message: "feedback registrado", Data: map[string]any{"id": 1, "athlete_user_id": 7}})
	var frame map[string]any
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, UpdateSetEventType, frame["type"])
	assert.Equal(t, "session:7", frame["channel"])
	assert.Equal(t, map[string]any{"message": "feedback registrado", "data": map[string]any{"id": 1.0, "athlete_user_id": 7.0}}, frame["data"])
}

// TestHubNotifierEmitDeliversBroadcast: HubNotifier acopla Notifier con el Hub.
// Emit es asíncrono, por eso la entrega se espera con Eventually.
func TestHubNotifierEmitDeliversBroadcast(t *testing.T) {
	hub := NewHub()
	c := newTestClient(7)
	hub.Subscribe("session:9", c)
	require.Equal(t, 1, hub.Count("session:9"))

	notifier := NewHubNotifier(hub)
	notifier.Emit("session:9", []byte(`{"type":"update:set_event","data":{"message":"feedback registrado","data":{"id":1}}}`))

	var got [][]byte
	require.Eventually(t, func() bool {
		got = append(got, c.drain(1)...)
		return len(got) >= 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, string(got[0]), `"type":"update:set_event"`)
}

// TestHubNotifierNoopCases: canal sin suscriptos y nil receiver son no-op.
func TestHubNotifierNoopCases(t *testing.T) {
	hub := NewHub()
	notifier := NewHubNotifier(hub)
	notifier.Emit("session:404", []byte(`{"n":1}`)) // canal sin suscriptos
	assert.Equal(t, 0, hub.Count("session:404"))

	// nil receiver: no-op (inyección opcional del controller).
	var nilNotifier *HubNotifier
	nilNotifier.Emit("session:7", []byte(`{}`))
	assert.Equal(t, 0, hub.Count("session:7"))
}

// TestNotifierEmitIsNonBlocking: Emit nunca se bloquea aunque el receptor esté
// lleno: el Hub descarta frames por overflow y la llamada vuelve enseguida.
func TestNotifierEmitIsNonBlocking(t *testing.T) {
	hub := NewHub()
	c := newTestClient(1)
	hub.Subscribe("session:1", c)

	notifier := NewHubNotifier(hub)
	for i := 0; i < sendBufferSize*2; i++ {
		start := time.Now()
		notifier.Emit("session:1", []byte(`{"i":1}`))
		require.Less(t, time.Since(start), 50*time.Millisecond, "Emit (%d) bloqueó", i)
	}
	// best-effort: llegan solo los del buffer, sin pánico ni deadlock.
	frames := waitFrames(c, sendBufferSize, 20)
	require.LessOrEqual(t, len(frames), sendBufferSize)
}

// TestUpdateSessionStatePayloadShape: el frame update:session_state (Gap 26
// D10) viaja con data = objeto de estado presencial post-write.
func TestUpdateSessionStatePayloadShape(t *testing.T) {
	raw := MarshalUpdateSessionState("session:11", map[string]any{"presencial_open": true, "closed_at": nil})
	var frame map[string]any
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, UpdateSessionStateEventType, frame["type"])
	assert.Equal(t, "session:11", frame["channel"])
	data := frame["data"].(map[string]any)
	assert.Equal(t, true, data["presencial_open"])
	v, hasClosed := data["closed_at"]
	require.True(t, hasClosed)
	assert.Nil(t, v)
}

// TestUpdateAttendanceEventPayloadShape: el frame update:attendance_event
// (Gap 28 D12) viaja con data = la fila del roster afectada, sin doble
// wrapper: source y registered_at en null es el contrato del borrado.
func TestUpdateAttendanceEventPayloadShape(t *testing.T) {
	type row struct {
		UserID       int64      `json:"user_id"`
		Status       string     `json:"status"`
		Source       *string    `json:"source"`
		RegisteredAt *time.Time `json:"registered_at"`
		AttendanceID *int64     `json:"attendance_id"`
	}
	registered := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	source := "qr"
	attID := int64(42)
	raw := MarshalUpdateAttendanceEvent("session:9", row{UserID: 7, Status: "attended", Source: &source, RegisteredAt: &registered, AttendanceID: &attID})
	var frame map[string]any
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, UpdateAttendanceEventType, frame["type"])
	assert.Equal(t, "session:9", frame["channel"])
	data := frame["data"].(map[string]any)
	assert.Equal(t, 7.0, data["user_id"])
	assert.Equal(t, "attended", data["status"])
	assert.Equal(t, "qr", data["source"])
	assert.Equal(t, 42.0, data["attendance_id"])

	// Baja: not_confirmed con los tres campos en null explícito.
	raw = MarshalUpdateAttendanceEvent("session:9", row{UserID: 7, Status: "not_confirmed"})
	require.NoError(t, json.Unmarshal(raw, &frame))
	data = frame["data"].(map[string]any)
	assert.Equal(t, "not_confirmed", data["status"])
	for _, key := range []string{"source", "registered_at", "attendance_id"} {
		v, present := data[key]
		require.True(t, present, "%s debe viajar aunque sea null", key)
		assert.Nil(t, v)
	}
}

// TestControlMessageCreatedPayloadShape: el frame `control:message_created`
// (Gap 27 D9) viaja con channel y un payload mínimo {sessionMessageId} — sin
// contenido del mensaje, la privacidad la aplica el filtro del GET.
func TestControlMessageCreatedPayloadShape(t *testing.T) {
	raw := MarshalControlMessageCreated("session:88", 9)
	var frame struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Payload struct {
			SessionMessageID float64 `json:"sessionMessageId"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, "control:message_created", frame.Type)
	assert.Equal(t, "session:88", frame.Channel)
	assert.Equal(t, 9.0, frame.Payload.SessionMessageID)
}
