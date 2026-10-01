package realtime

import "sync"

// ChannelAuthorizer decide si userID puede suscribirse a channel. Se inyecta
// desde el wire-up: los patrones de canal (p.ej. session:{id}) no viven en
// este paquete.
type ChannelAuthorizer interface {
	Authorize(channel string, userID int64) (bool, error)
}

const (
	sendBufferSize = 32
	// overflowLimit: envíos consecutivos descartados que cierran la conexión.
	overflowLimit = 8
)

// client es una conexión suscripta. El overflow sostenido dispara drop una
// sola vez por episodio (un envío exitoso desarma la marca).
type client struct {
	userID int64
	// send nunca se cierra mientras el cliente pueda estar suscripto: el
	// dueño debe desuscribir de todos los canales ANTES de close(send), o
	// Broadcast puede enviar sobre un canal cerrado.
	send chan []byte

	mu          sync.Mutex
	overflowing int
	dropFired   bool
	drop        func() // no-bloqueante; dueño de la conexión decide cómo cerrar
}

func newClient(userID int64, drop func()) *client {
	return &client{userID: userID, send: make(chan []byte, sendBufferSize), drop: drop}
}

// enqueue es no-bloqueante; descarta el frame si el buffer está lleno.
// Devuelve true si la conexión quedó marcada para cerrar (overflow sostenido).
func (c *client) enqueue(frame []byte) bool {
	var fire func()
	select {
	case c.send <- frame:
		c.mu.Lock()
		c.overflowing = 0
		c.dropFired = false
		c.mu.Unlock()
		return false
	default:
		c.mu.Lock()
		c.overflowing++
		overflowed := c.overflowing >= overflowLimit
		if overflowed && !c.dropFired {
			c.dropFired = true
			fire = c.drop
		}
		c.mu.Unlock()
		if fire != nil {
			fire()
		}
		return overflowed
	}
}

func (c *client) drain(limit int) [][]byte {
	frames := make([][]byte, 0, limit)
	for i := 0; i < limit; i++ {
		var raw []byte
		select {
		case raw = <-c.send:
			frames = append(frames, raw)
		default:
			return frames
		}
	}
	return frames
}

// Hub es el registro canal → suscriptos. Todas las operaciones son seguras
// para uso concurrente; los eventos a canales sin suscriptos son no-op.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

// NewHub inicializa el registro.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*client]struct{})}
}

// Subscribe agrega c al set de channel.
func (h *Hub) Subscribe(channel string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.rooms[channel]
	if !ok {
		set = make(map[*client]struct{})
		h.rooms[channel] = set
	}
	set[c] = struct{}{}
}

// Unsubscribe saca c de channel y borra el canal si quedó vacío. Idempotente.
func (h *Hub) Unsubscribe(channel string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.rooms[channel]
	if !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.rooms, channel)
	}
}

// Broadcast encola frame en cada suscripto de channel excepto exclude. Es
// no-bloqueante: descarta el frame para el cliente cuyo buffer está lleno
// (feed best-effort); si el overflow persiste, dispara el drop del cliente.
func (h *Hub) Broadcast(channel string, frame []byte, exclude *client) {
	h.mu.RLock()
	set, ok := h.rooms[channel]
	if !ok || len(set) == 0 {
		h.mu.RUnlock()
		return
	}
	targets := make([]*client, 0, len(set))
	for c := range set {
		if exclude != nil && c == exclude {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.enqueue(frame)
	}
}

// Count devuelve cuántos clientes hay suscriptos a channel ahora mismo.
func (h *Hub) Count(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[channel])
}
