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
	raw := MarshalUpdateSetEvent(Remote{Message: "feedback registrado", Data: map[string]any{"id": 1, "athlete_user_id": 7}})
	var frame map[string]any
	require.NoError(t, json.Unmarshal(raw, &frame))
	assert.Equal(t, UpdateSetEventType, frame["type"])
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
