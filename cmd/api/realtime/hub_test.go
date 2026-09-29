package realtime

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(userID int64) *client {
	return newClient(userID, nil)
}

// waitFrames trae hasta n frames del canal de salida de c. Mientras el
// productor puede seguir metiendo frames, un solo drain con `default` puede
// volver vacío si corre antes del broadcast; por eso se reintenta.
func waitFrames(c *client, n, retries int) [][]byte {
	var frames [][]byte
	for i := 0; i < retries && len(frames) < n; i++ {
		frames = append(frames, c.drain(n-len(frames))...)
	}
	return frames
}

func TestBroadcastReachesAllExceptExcluded(t *testing.T) {
	hub := NewHub()
	a, b, sender := newTestClient(1), newTestClient(2), newTestClient(3)
	hub.Subscribe("session:1", a)
	hub.Subscribe("session:1", b)

	sent := MarshalOutbound(&outboundMessage{Type: TypePresence, From: 3, Payload: []byte(`{"here":true}`)})
	hub.Broadcast("session:1", sent, sender)

	frames := waitFrames(b, 1, 50)
	require.Len(t, frames, 1)
	assert.Contains(t, string(frames[0]), `"from":3`)

	// Ni el excluido ni nadie fuera del canal reciben.
	assert.Empty(t, sender.drain(8))
}

func TestBroadcastNoSubscribersIsNoop(t *testing.T) {
	hub := NewHub()
	hub.Broadcast("session:404", []byte(`{"type":"presence"}`), nil)
	assert.Equal(t, 0, hub.Count("session:404"))
}

func TestCount(t *testing.T) {
	hub := NewHub()
	a, b := newTestClient(1), newTestClient(2)
	assert.Equal(t, 0, hub.Count("session:7"))
	hub.Subscribe("session:7", a)
	hub.Subscribe("session:7", a)
	assert.Equal(t, 1, hub.Count("session:7"))
	hub.Subscribe("session:7", b)
	hub.Subscribe("session:8", a)
	assert.Equal(t, 2, hub.Count("session:7"))
	assert.Equal(t, 1, hub.Count("session:8"))
}

func TestUnsubscribeRemovesAndCleanups(t *testing.T) {
	hub := NewHub()
	a, b := newTestClient(1), newTestClient(2)
	hub.Subscribe("session:9", a)
	hub.Subscribe("session:9", b)
	hub.Unsubscribe("session:9", a)
	assert.Equal(t, 1, hub.Count("session:9"))

	hub.Unsubscribe("session:9", b)
	assert.Equal(t, 0, hub.Count("session:9"))
	// Idempotente: desuscribir de más no rompe.
	hub.Unsubscribe("session:9", b)
	assert.Equal(t, 0, hub.Count("session:9"))
	hub.Unsubscribe("canal-nunca-existio", a)
}

func TestBroadcastStopsAfterUnsubscribe(t *testing.T) {
	hub := NewHub()
	a, b := newTestClient(1), newTestClient(2)
	hub.Subscribe("session:2", a)
	hub.Subscribe("session:2", b)
	hub.Unsubscribe("session:2", a)

	hub.Broadcast("session:2", []byte(`{"x":1}`), nil)
	assert.Empty(t, a.drain(8))
	assert.Len(t, waitFrames(b, 1, 50), 1)
}

func TestOverflowDiscardsAndFiresDropOnce(t *testing.T) {
	hub := NewHub()
	var dropCalls atomic.Int32
	c := newClient(7, func() { dropCalls.Add(1) })
	hub.Subscribe("session:3", c)

	frame := []byte(`{"type":"presence"}`)
	fill := func() {
		for i := 0; i < sendBufferSize; i++ {
			require.False(t, c.enqueue(frame))
		}
	}

	// Se llena el buffer: todos los envíos entran y no marcan overflow.
	fill()
	require.Len(t, c.drain(sendBufferSize), sendBufferSize)

	// Se re-llena; con el buffer lleno, los envíos siguientes descartan.
	fill()
	// overflowLimit-1 descartes: aún no cierra.
	for i := 0; i < overflowLimit-1; i++ {
		assert.False(t, c.enqueue(frame))
	}
	assert.Equal(t, int32(0), dropCalls.Load())

	// El descarte que alcanza el límite dispara drop una sola vez.
	assert.True(t, c.enqueue(frame))
	assert.Equal(t, int32(1), dropCalls.Load())

	// Broadcast con buffer lleno sigue descartando sin re-disparar drop.
	hub.Broadcast("session:3", frame, nil)
	assert.Equal(t, int32(1), dropCalls.Load())

	// Drenar + un envío exitoso desarma la marca; un nuevo episodio re-dispara.
	c.drain(2 * sendBufferSize)
	assert.False(t, c.enqueue(frame))
	fill()
	for i := 0; i < overflowLimit; i++ {
		c.enqueue(frame)
	}
	assert.Equal(t, int32(2), dropCalls.Load())
}

func TestConcurrentSubscribeBroadcastUnsubscribe(t *testing.T) {
	hub := NewHub()
	const channel = "session:race"
	const workers = 8
	const clientsPerWorker = 25

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < clientsPerWorker; i++ {
				c := newTestClient(int64(worker*clientsPerWorker + i))
				hub.Subscribe(channel, c)
				hub.Broadcast(channel, []byte(fmt.Sprintf(`{"i":%d}`, i)), nil)
				if i%2 == 0 {
					hub.Unsubscribe(channel, c)
				}
			}
		}(w)
	}
	wg.Wait()

	// Quedan suscriptos los clientes que no pasaron por unsubscribe.
	assert.Equal(t, workers*(clientsPerWorker/2), hub.Count(channel))
}

func TestSameFrameFanout(t *testing.T) {
	hub := NewHub()
	cs := make([]*client, 5)
	for i := range cs {
		cs[i] = newTestClient(int64(i + 1))
		hub.Subscribe("session:fan", cs[i])
	}
	frame := MarshalOutbound(&outboundMessage{Type: TypePong})
	hub.Broadcast("session:fan", frame, nil)
	for _, c := range cs {
		frames := waitFrames(c, 1, 50)
		require.Len(t, frames, 1)
		assert.Equal(t, frame, frames[0])
	}
}
