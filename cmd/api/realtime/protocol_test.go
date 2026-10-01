package realtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInbound(t *testing.T) {
	t.Run("subscribe válido", func(t *testing.T) {
		msg, err := ParseInbound([]byte(`{"type":"subscribe","channel":"session:123"}`))
		require.NoError(t, err)
		assert.Equal(t, TypeSubscribe, msg.Type)
		assert.Equal(t, "session:123", msg.Channel)
	})

	t.Run("presence con payload opaco", func(t *testing.T) {
		msg, err := ParseInbound([]byte(`{"type":"presence","payload":{"status":"moving"}}`))
		require.NoError(t, err)
		assert.Equal(t, TypePresence, msg.Type)
		assert.JSONEq(t, `{"status":"moving"}`, string(msg.Payload))
	})

	t.Run("ping", func(t *testing.T) {
		msg, err := ParseInbound([]byte(`{"type":"ping"}`))
		require.NoError(t, err)
		assert.Equal(t, TypePing, msg.Type)
	})

	t.Run("subscribe sin channel", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"subscribe"}`))
		require.Error(t, err)
	})

	t.Run("presence con payload null", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"presence","payload":null}`))
		require.Error(t, err)
	})

	t.Run("presence con payload arreglo", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"presence","payload":[1,2]}`))
		require.Error(t, err)
	})

	t.Run("control con payload arreglo", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"control","payload":["x"]}`))
		require.Error(t, err)
	})

	t.Run("presence sin payload", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"presence"}`))
		require.Error(t, err)
	})

	t.Run("tipo desconocido", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{"type":"hack","channel":"x"}`))
		require.Error(t, err)
	})

	t.Run("JSON inválido", func(t *testing.T) {
		_, err := ParseInbound([]byte(`{`))
		require.Error(t, err)
	})
}

func TestMarshalOutbound(t *testing.T) {
	t.Run("frame de suscripción", func(t *testing.T) {
		raw := MarshalOutbound(&outboundMessage{Type: TypeSubscribed, Channel: "session:123"})
		assert.JSONEq(t, `{"type":"subscribed","channel":"session:123"}`, string(raw))
	})

	t.Run("update:set_event con Remote", func(t *testing.T) {
		raw := MarshalOutbound(&outboundMessage{
			Type: UpdateSetEventType,
			Data: Remote{Message: "ok", Data: map[string]any{"id": 1}},
		})
		assert.JSONEq(t, `{"type":"update:set_event","data":{"message":"ok","data":{"id":1}}}`, string(raw))
	})

	t.Run("data no serializable cae a frame de error", func(t *testing.T) {
		raw := MarshalOutbound(&outboundMessage{Type: TypeUpdate, Data: make(chan int)})
		var frame outboundMessage
		require.NoError(t, json.Unmarshal(raw, &frame))
		assert.Equal(t, TypeError, frame.Type)
		assert.NotEmpty(t, frame.Message)
	})
}
