package realtime

// Notifier es el emisor genérico de eventos server→client: quien decide el
// nombre del canal es el caller (el paquete no conoce ningún patrón de canal).
// Emit debe ser no-bloqueante y async-safe; el payload no debe mutarse
// después de Emit (el fan-out es async). Un valor nil es válido y hace de
// la llamada un no-op para que el wiring opcional no exija chequeos en el hook.
type Notifier interface {
	Emit(channel string, payload []byte)
}

// Compile-time check: HubNotifier debe seguir cumpliendo Notifier.
var _ Notifier = (*HubNotifier)(nil)

// HubNotifier adapta el Hub a Notifier. Mantiene la semántica best-effort del
// Hub: canal sin suscriptos o buffer lleno → frame descartado, sin error.
type HubNotifier struct {
	hub *Hub
}

func NewHubNotifier(hub *Hub) *HubNotifier {
	return &HubNotifier{hub: hub}
}

func (n *HubNotifier) Emit(channel string, payload []byte) {
	if n == nil || n.hub == nil || channel == "" || len(payload) == 0 {
		return
	}
	// Asíncrono: el fan-out corre fuera del request que emite (best-effort;
	// el Hub descarta si el buffer del receptor está lleno).
	go n.hub.Broadcast(channel, payload, nil)
}

// MarshalUpdateSetEvent arma el frame `update:set_event` (D4): data es el
// objeto que el endpoint devuelve en su body HTTP (p.ej. MutationResponse),
// de modo que el wrapper {message, data} replica el body exacto y el frontend
// reutiliza su normalizador.
func MarshalUpdateSetEvent(data any) []byte {
	return MarshalOutbound(&outboundMessage{
		Type: UpdateSetEventType,
		Data: data,
	})
}
