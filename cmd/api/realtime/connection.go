package realtime

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// maxMessageSize acota frames cliente→servidor (presence/control opacos).
	maxMessageSize = 4096
	// writeWait: deadline de escritura por frame saliente.
	writeWait = 10 * time.Second
	// readWait refrescado en cada frame leído; el heartbeat JSON del frontend
	// (20-30s) mantiene viva la conexión sin ping a nivel TCP.
	readWait = 45 * time.Second
	// maxChannelsPerConn: tope de suscripciones vivas por conexión.
	maxChannelsPerConn = 20
)

// Gateway upgradea y sirve conexiones WebSocket sobre el Hub. Sin gin y sin
// dominio: el 401 por token lo resuelve el caller antes del upgrade y la
// autorización de canales se inyecta — este paquete solo transporta.
type Gateway struct {
	hub            *Hub
	authorizer     ChannelAuthorizer
	allowedOrigins map[string]bool
}

func NewGateway(hub *Hub, authorizer ChannelAuthorizer, allowedOrigins []string) *Gateway {
	originMap := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originMap[o] = true
	}
	return &Gateway{hub: hub, authorizer: authorizer, allowedOrigins: originMap}
}

// Upgrade valida el Origin (los clientes nativos no envían el header y pasan)
// y hace el handshake. El 401 por token ausente/inválido lo resuelve el caller
// ANTES de llamar acá, así cada lado escribe su propio layout de error (gin
// JSON en app, ResponseWriter directo en tests).
func (g *Gateway) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || g.allowedOrigins[origin] || sameHost(origin, r.Host)
	}}
	return upgrader.Upgrade(w, r, nil)
}

// sameHost acepta clientes nativos (React Native manda como Origin la URL
// del server al que conecta): si el host:puerto del origin coincide con el
// Host del request, es un cliente legítimo a esta misma instancia. Cubre
// LAN/dispositivos físicos sin mantener IPs en env ni código.
func sameHost(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == host
}

// Serve atiende conn ya autenticada como userID hasta que se caiga (read
// error, close del par o drop por overflow: el drop cierra la conexión por
// callback). Read pump inline + write pump en goroutine propia. Al salir
// limpia en orden: señal de fin al pump, desuscribir TODOS los canales (el
// pump deja de escribir, puede seguir drenando lo encolado sin destino) y
// close de la conexión — nunca se cierra send: encoladores que sobreviven a
// la limpieza (Hub en carrera con Broadcast) escribirían sobre canal cerrado.
func (g *Gateway) Serve(conn *websocket.Conn, userID int64) {
	sc := &serveState{
		client:   newClient(userID, func() { _ = conn.Close() }),
		conn:     conn,
		channels: make(map[string]struct{}, maxChannelsPerConn),
		done:     make(chan struct{}),
	}
	go g.writePump(sc)

	defer func() {
		close(sc.done)
		for ch := range sc.channels {
			g.hub.Unsubscribe(ch, sc.client)
		}
		_ = conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(readWait))

		msg, err := ParseInbound(raw)
		if err != nil {
			sc.enqueue(MarshalOutbound(errOutbound(err.Error())))
			continue
		}

		g.apply(sc, msg)
	}
}

// serveState es el estado de UNA conexión en serve: el client del Hub + el
// set de canales suscriptos en esta conexión y la señal de salida coordinada.
type serveState struct {
	*client
	conn     *websocket.Conn
	channels map[string]struct{}
	done     chan struct{}
}

// apply ejecuta el frame ya validado. Los rechazos salen como frame `error` y
// el loop sigue: la única salida es un error de read/write del socket (D6).
// TODA escritura pasa por la cola del pump: gorilla admite un único writer
// concurrente, y el pump es el único que toca el socket para escribir.
func (g *Gateway) apply(sc *serveState, msg *clientMessage) {
	switch msg.Type {
	case TypeSubscribe:
		g.handleSubscribe(sc, msg.Channel)
	case TypeUnsubscribe:
		g.hub.Unsubscribe(msg.Channel, sc.client)
		delete(sc.channels, msg.Channel)
	case TypePresence, TypeControl:
		channel, reason := relayChannel(msg, sc.channels)
		if channel == "" {
			sc.enqueue(MarshalOutbound(errOutbound(reason)))
			return
		}
		g.hub.Broadcast(channel, MarshalOutbound(&outboundMessage{Type: msg.Type, From: sc.userID, Payload: msg.Payload}), sc.client)
	case TypePing:
		sc.enqueue(MarshalOutbound(&outboundMessage{Type: TypePong}))
	default:
		sc.enqueue(MarshalOutbound(errOutbound("tipo desconocido: " + msg.Type)))
	}
}

// relayChannel resuelve el canal de reenvío de presence/control: los frames
// del D4 no traen channel explícito — se usa el campo channel del frame si
// viene (multi-canal) o la única suscripción activa de la conexión; ningún
// canal resoluble → error sin cortar.
func relayChannel(msg *clientMessage, channels map[string]struct{}) (string, string) {
	if msg.Channel != "" {
		if _, subscribed := channels[msg.Channel]; !subscribed {
			return "", "no suscripto al canal " + msg.Channel
		}
		return msg.Channel, ""
	}
	if len(channels) == 1 {
		for subscribed := range channels {
			return subscribed, ""
		}
	}
	return "", "presence/control requieren un canal suscripto (channel o única suscripción)"
}

// handleSubscribe valida tope y autorización antes de suscribir. El resub de
// un canal propio es idempotente (responde `subscribed`) y se chequea ANTES
// del tope. Tope alcanzado, canal ajeno o desconocido responden `error` sin
// cortar.
func (g *Gateway) handleSubscribe(sc *serveState, channel string) {
	if _, dup := sc.channels[channel]; dup {
		sc.enqueue(MarshalOutbound(&outboundMessage{Type: TypeSubscribed, Channel: channel}))
		return
	}
	if len(sc.channels) >= maxChannelsPerConn {
		sc.enqueue(MarshalOutbound(errOutbound("tope de canales por conexión alcanzado")))
		return
	}

	allowed, err := g.authorizer.Authorize(channel, sc.userID)
	if err != nil {
		sc.enqueue(MarshalOutbound(errOutbound("error verificando acceso al canal")))
		return
	}
	if !allowed {
		sc.enqueue(MarshalOutbound(errOutbound("no autorizado al canal " + channel)))
		return
	}

	sc.channels[channel] = struct{}{}
	g.hub.Subscribe(channel, sc.client)
	sc.enqueue(MarshalOutbound(&outboundMessage{Type: TypeSubscribed, Channel: channel}))
}

// writePump es el ÚNICO writer del socket: drena el canal de salida del
// client con deadline por frame. Corte: write falla (conn caída), señal done
// (limpieza de Serve) o canal cerrado (nunca debería: Serve no cierra send).
func (g *Gateway) writePump(sc *serveState) {
	for {
		select {
		case frame, ok := <-sc.send:
			if !ok {
				return
			}
			if frame != nil && !writeToConn(sc.conn, frame) {
				return
			}
		case <-sc.done:
			return
		}
	}
}

// writeToConn escribe un frame con deadline por flush; false = conn muerta
// (el caller puede dejar de encolar/leer: la limpieza la hace Serve).
func writeToConn(conn *websocket.Conn, frame []byte) bool {
	if frame == nil {
		return true
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
		_ = conn.Close()
		return false
	}
	return true
}
