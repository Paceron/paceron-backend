// Package realtime: hub de conexión en memoria para el gateway WebSocket y
// su protocolo JSON (un objeto por frame). El paquete no conoce dominio: la
// autorización de canales se inyecta vía ChannelAuthorizer.
package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Tipos de mensajes cliente → servidor.
const (
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypePresence    = "presence"
	TypeControl     = "control"
	TypePing        = "ping"
)

// Tipos de mensajes servidor → cliente.
const (
	TypeSubscribed = "subscribed"
	TypeError      = "error"
	TypePong       = "pong"
	TypeUpdate     = "update"
)

// EventSetEvent identifica el evento server-originado de feedback de sesión
// (se compone como "update:<event>").
const EventSetEvent = "set_event"

// UpdateSetEventType = "update:set_event" (diseño D4).
const UpdateSetEventType = TypeUpdate + ":" + EventSetEvent

// clientMessage es el decode de todo frame cliente → servidor. payload de
// presence/control viaja opaco pero debe ser un objeto JSON (ver ParseInbound).
type clientMessage struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// outboundMessage es el encode de todo frame servidor → cliente. Un único
// struct con omitempty cubre todos los frames del protocolo.
type outboundMessage struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	From    int64           `json:"from,omitempty"` // userID emisor en presence/control
	Message string          `json:"message,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"` // presence/control
	Data    any             `json:"data,omitempty"`    // update:set_event
}

// Remote repite el wrapper de MutationResponse para que el gateway pueda
// armar update:set_event sin importar la capa HTTP. En Create se envía
// workoutfeedback.MutationResponse tal cual va en la respuesta HTTP.
type Remote struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// errOutbound devuelve un frame `error` genérico.
func errOutbound(msg string) *outboundMessage {
	return &outboundMessage{Type: TypeError, Message: msg}
}

// MarshalOutbound serializa un frame servidor → cliente. Si el payload no
// serializa, cae a un frame de error: el cliente nunca queda esperando un
// evento que no puede representarse.
func MarshalOutbound(frame *outboundMessage) []byte {
	raw, err := json.Marshal(frame)
	if err == nil {
		return raw
	}
	fallback, ferr := json.Marshal(errOutbound("error interno del gateway"))
	if ferr == nil {
		return fallback
	}
	return nil
}

// ParseInbound decodifica y valida un frame cliente → servidor.
func ParseInbound(raw []byte) (*clientMessage, error) {
	var msg clientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("frame inválido: %w", err)
	}
	switch msg.Type {
	case TypeSubscribe, TypeUnsubscribe:
		if msg.Channel == "" {
			return nil, errors.New(msg.Type + " requiere channel")
		}
	case TypePresence, TypeControl:
		if len(msg.Payload) == 0 || !json.Valid(msg.Payload) {
			return nil, errors.New(msg.Type + " requiere payload JSON")
		}
		// El payload debe decodificar a objeto JSON: el backend reenvía pero
		// no transmite túneles de null/arreglos (Unmarshal de null no falla
		// pero deja el map nil).
		var obj map[string]any
		if err := json.Unmarshal(msg.Payload, &obj); err != nil || obj == nil {
			return nil, errors.New(msg.Type + " requiere payload objeto JSON")
		}
	case TypePing:
		return &msg, nil
	default:
		return nil, fmt.Errorf("tipo desconocido: %q", msg.Type)
	}
	return &msg, nil
}
