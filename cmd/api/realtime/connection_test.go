package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/utils"
)

// fakeAuthorizer fija la firma de ChannelAuthorizer (deferred minor T1) y
// autoriza por tabla: session:{sub} → {userID: true}.
type fakeAuthorizer struct {
	allowed map[string]map[int64]bool
	failOn  map[string]bool
}

var _ ChannelAuthorizer = (*fakeAuthorizer)(nil)

// fakeFullAuthorizer autoriza session:7 SOLO a userID (canal ajeno = otro user).
func fakeFullAuthorizer(userID int64) *fakeAuthorizer {
	return &fakeAuthorizer{allowed: map[string]map[int64]bool{"7": {userID: true}}}
}

func (f *fakeAuthorizer) Authorize(channel string, userID int64) (bool, error) {
	if f.failOn[channel] {
		return false, errors.New("fallo de infraestructura simulado")
	}
	sub := strings.TrimPrefix(channel, "session:")
	return f.allowed[sub][userID], nil
}

// lenientAuthorizer autoriza cualquier session:{id>0} (para el test de tope).
type lenientAuthorizer struct{}

func (lenientAuthorizer) Authorize(channel string, _ int64) (bool, error) {
	sub, ok := strings.CutPrefix(channel, "session:")
	if !ok || sub == "" {
		return false, nil
	}
	var id int64
	if _, err := fmt.Sscanf(sub, "%d", &id); err != nil || id <= 0 {
		return false, nil
	}
	return true, nil
}

// gatewayUpgradeHandler replica el wiring de app (token→401→Upgrade→Serve)
// sin depender del paquete app; el flujo gin completo se prueba arriba.
func gatewayUpgradeHandler(gateway *Gateway) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"code": "unauthorized", "message": "falta el query param token"})
			return
		}
		claims, err := utils.ParseAccessToken(token)
		if err != nil {
			code, message := "unauthorized", "token inválido"
			if errors.Is(err, jwt.ErrTokenExpired) {
				code, message = "token_expired", "el access token expiró"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"code": code, "message": message})
			return
		}
		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"code": "unauthorized", "message": "token inválido"})
			return
		}
		conn, err := gateway.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		gateway.Serve(conn, userID)
	}
}

func setGatewayJWTConfig(t *testing.T) {
	t.Helper()
	oldSecret, oldIssuer, oldAudience, oldDuration := config.JWTSecret, config.JWTIssuer, config.JWTAudience, config.AccessTokenDuration
	t.Cleanup(func() {
		config.JWTSecret, config.JWTIssuer, config.JWTAudience, config.AccessTokenDuration = oldSecret, oldIssuer, oldAudience, oldDuration
	})
	config.JWTSecret = "test-secret-key-for-testing"
	config.JWTIssuer = "paceron-backend"
	config.JWTAudience = "paceron-app"
	config.AccessTokenDuration = 15 * time.Minute
}

type wsTestServer struct {
	httptestServer *httptest.Server
	hub            *Hub
	wsURL          string
}

func newWSUpgradeServer(t *testing.T, authorizer ChannelAuthorizer) wsTestServer {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	hub := NewHub()
	gateway := NewGateway(hub, authorizer, []string{"http://allowed-origin.example"})
	router := gin.New()
	router.GET("/api/v1/ws", gatewayUpgradeHandler(gateway))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return wsTestServer{httptestServer: server, hub: hub, wsURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws"}
}

func tok(t *testing.T, userID int64) string {
	t.Helper()
	token, err := utils.GenerateAccessToken(userID, "sess-test", []string{"corredor"})
	require.NoError(t, err)
	return token
}

type wsFrame struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Message string          `json:"message,omitempty"`
	From    int64           `json:"from,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type wsTestClient struct {
	conn   *websocket.Conn
	frames chan wsFrame
	errs   chan error
}

func dialWS(t *testing.T, url string, header http.Header) *wsTestClient {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := &wsTestClient{conn: conn, frames: make(chan wsFrame, 32), errs: make(chan error, 4)}
	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, raw, err := conn.ReadMessage()
			if err != nil {
				client.errs <- err
				return
			}
			var f wsFrame
			if uerr := json.Unmarshal(raw, &f); uerr != nil {
				continue
			}
			client.frames <- f
		}
	}()
	return client
}

func (w *wsTestClient) send(t *testing.T, frame map[string]any) {
	t.Helper()
	_ = w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, w.conn.WriteJSON(frame))
}

func (w *wsTestClient) sendRaw(t *testing.T, raw []byte) {
	t.Helper()
	_ = w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, w.conn.WriteMessage(websocket.TextMessage, raw))
}

func (w *wsTestClient) waitFrame(t *testing.T, typ string) wsFrame {
	t.Helper()
	select {
	case f := <-w.frames:
		require.Equal(t, typ, f.Type, "frame inesperado: %+v", f)
		return f
	case err := <-w.errs:
		t.Fatalf("la conexión se cortó esperando %q: %v", typ, err)
		return wsFrame{}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout esperando frame %q", typ)
		return wsFrame{}
	}
}

// waitSilence aserta que en el plazo NO llega ningún frame (emisor excluido,
// desuscripto, canal sin otros suscriptos).
func (w *wsTestClient) waitSilence(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case f := <-w.frames:
		t.Fatalf("llegó frame inesperado durante el silencio: %+v", f)
	case err := <-w.errs:
		t.Fatalf("la conexión se cortó durante el silencio: %v", err)
	case <-time.After(d):
	}
}

// expectDisconnect aserta que la conexión muere (close frame/cut del server).
func (w *wsTestClient) expectDisconnect(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case f := <-w.frames:
		t.Fatalf("llegó frame %+v, se esperaba corte", f)
	case err := <-w.errs:
		require.Error(t, err)
	case <-time.After(d):
		t.Fatal("timeout esperando el corte de la conexión")
	}
}

func pingPong(t *testing.T, w *wsTestClient) {
	t.Helper()
	w.send(t, map[string]any{"type": TypePing})
	assert.Equal(t, TypePong, w.waitFrame(t, TypePong).Type)
}

func subscribe(w *wsTestClient, t *testing.T, channel string) {
	t.Helper()
	w.send(t, map[string]any{"type": TypeSubscribe, "channel": channel})
}

func TestUpgradeWithoutTokenReturns401JSON(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))

	resp, err := http.Get(strings.Replace(server.wsURL, "ws://", "http://", 1))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "unauthorized", body["code"])
	assert.Contains(t, body["message"], "falta el query param token")
}

func TestUpgradeWithInvalidTokenReturns401JSON(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))

	resp, err := http.Get(strings.Replace(server.wsURL, "ws://", "http://", 1) + "?token=esto-no-es-un-token")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "unauthorized", body["code"])
	assert.Equal(t, "token inválido", body["message"])
}

func TestUpgradeWithExpiredTokenReturnsTokenExpired(t *testing.T) {
	setGatewayJWTConfig(t)
	oldDuration := config.AccessTokenDuration
	config.AccessTokenDuration = -1 * time.Minute
	token, err := utils.GenerateAccessToken(1, "sess-test", nil)
	require.NoError(t, err)
	config.AccessTokenDuration = oldDuration

	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	resp, err := http.Get(strings.Replace(server.wsURL, "ws://", "http://", 1) + "?token=" + token)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "token_expired", body["code"])
	assert.Equal(t, "el access token expiró", body["message"])
}

func TestUpgradeSucceedsWithValidTokenAndNoOrigin(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))

	// Los clientes nativos no mandan header Origin: el dial default no lo envía.
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	pingPong(t, client)
	assert.Equal(t, 0, server.hub.Count("session:7"))
}

func TestUpgradeRejectsDisallowedOrigin(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))

	header := http.Header{}
	header.Set("Origin", "http://origen-malingo.example")
	_, resp, err := websocket.DefaultDialer.Dial(server.wsURL+"?token="+tok(t, 1), header)
	require.Error(t, err)
	require.Equal(t, websocket.ErrBadHandshake, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpgradeAllowsConfiguredOrigin(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))

	header := http.Header{}
	header.Set("Origin", "http://allowed-origin.example")
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), header)
	pingPong(t, client)
}

func TestSubscribeAuthorizedRepliesSubscribedIdempotent(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(client, t, "session:7")
	frame := client.waitFrame(t, TypeSubscribed)
	assert.Equal(t, "session:7", frame.Channel)
	assert.Equal(t, 1, server.hub.Count("session:7"))

	// Resubscribe idempotente: nuevo subscribed, la suscripción sigue única.
	subscribe(client, t, "session:7")
	assert.Equal(t, "session:7", client.waitFrame(t, TypeSubscribed).Channel)
	assert.Equal(t, 1, server.hub.Count("session:7"))
}

func TestSubscribeForeignChannelErrorsAndKeepsConnAlive(t *testing.T) {
	setGatewayJWTConfig(t)
	// El fake autoriza session:7 SOLO a user 1: user 2 es ajeno.
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 2), http.Header{})

	subscribe(client, t, "session:7")
	frame := client.waitFrame(t, TypeError)
	assert.NotEmpty(t, frame.Message)
	assert.Equal(t, 0, server.hub.Count("session:7"))

	// La conexión sigue viva tras el error.
	pingPong(t, client)
}

func TestSubscribeUnknownOrInvalidChannelErrors(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	for _, channel := range []string{"team:9", "session:abc", "session:0", "session:-3"} {
		client.send(t, map[string]any{"type": TypeSubscribe, "channel": channel})
		assert.NotEmpty(t, client.waitFrame(t, TypeError).Message, "canal %q", channel)
	}

	pingPong(t, client)
}

func TestSubscribeWithAuthorizerFailureErrorsWithoutCutting(t *testing.T) {
	setGatewayJWTConfig(t)
	authorizer := fakeFullAuthorizer(1)
	authorizer.failOn = map[string]bool{"session:6": true}
	server := newWSUpgradeServer(t, authorizer)
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(client, t, "session:6")
	assert.NotEmpty(t, client.waitFrame(t, TypeError).Message)
	pingPong(t, client)
}

func TestPingRepliesPong(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	pingPong(t, client)
}

func TestInvalidFrameErrorsWithoutCutting(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	// JSON inválido.
	client.sendRaw(t, []byte(`{"type":`))
	assert.NotEmpty(t, client.waitFrame(t, TypeError).Message)
	// Tipo desconocido.
	client.send(t, map[string]any{"type": "hack", "channel": "session:7"})
	assert.NotEmpty(t, client.waitFrame(t, TypeError).Message)

	pingPong(t, client)
}

func TestPresenceReachesOthersExcludingSender(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	sender := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	peer := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(sender, t, "session:7")
	subscribe(peer, t, "session:7")
	sender.waitFrame(t, TypeSubscribed)
	peer.waitFrame(t, TypeSubscribed)
	require.Equal(t, 2, server.hub.Count("session:7"))

	sender.send(t, map[string]any{"type": TypePresence, "payload": map[string]any{"status": "moving", "km": 3.2}})
	frame := peer.waitFrame(t, TypePresence)
	assert.Equal(t, int64(1), frame.From)
	assert.JSONEq(t, `{"status":"moving","km":3.2}`, string(frame.Payload))

	// El emisor no recibe su propio frame.
	sender.waitSilence(t, 300*time.Millisecond)
}

func TestPresenceWithNoOtherSubscribersIsSilentNoop(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(client, t, "session:7")
	client.waitFrame(t, TypeSubscribed)

	client.send(t, map[string]any{"type": TypePresence, "payload": map[string]any{"status": "moving"}})
	client.waitSilence(t, 300*time.Millisecond)
	pingPong(t, client)
}

func TestRelayChannelResolution(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, lenientAuthorizer{})
	// Dos suscripciones + channel explícito del frame resuelve el canal.
	pepe := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	peer := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	for _, ch := range []string{"session:7", "session:8"} {
		pepe.send(t, map[string]any{"type": TypeSubscribe, "channel": ch})
		pepe.waitFrame(t, TypeSubscribed)
	}
	subscribe(peer, t, "session:7")
	peer.waitFrame(t, TypeSubscribed)

	// channel explícito y suscripto: reenvío al canal indicado.
	pepe.send(t, map[string]any{"type": TypePresence, "channel": "session:7", "payload": map[string]any{"n": 1}})
	assert.Equal(t, int64(1), peer.waitFrame(t, TypePresence).From)

	// channel explícito al que NO está suscripto: error sin cortar.
	pepe.send(t, map[string]any{"type": TypePresence, "channel": "session:9", "payload": map[string]any{"n": 2}})
	assert.Contains(t, pepe.waitFrame(t, TypeError).Message, "no suscripto")
	peer.send(t, map[string]any{"type": TypePing})
	assert.Equal(t, TypePong, peer.waitFrame(t, TypePong).Type)
}

func TestPresencePayloadMustBeJSONObject(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	sender := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	peer := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(sender, t, "session:7")
	subscribe(peer, t, "session:7")
	sender.waitFrame(t, TypeSubscribed)
	peer.waitFrame(t, TypeSubscribed)

	// payload null → error, conexión viva.
	sender.sendRaw(t, []byte(`{"type":"presence","payload":null}`))
	assert.Contains(t, sender.waitFrame(t, TypeError).Message, "objeto JSON")

	// payload arreglo → error, conexión viva.
	sender.sendRaw(t, []byte(`{"type":"presence","payload":[1]}`))
	assert.Contains(t, sender.waitFrame(t, TypeError).Message, "objeto JSON")
	assert.Equal(t, 2, server.hub.Count("session:7")) // nada reenviado

	// payload objeto → reenvío normal.
	sender.send(t, map[string]any{"type": TypePresence, "payload": map[string]any{"n": 1}})
	assert.Equal(t, int64(1), peer.waitFrame(t, TypePresence).From)

	pingPong(t, sender)
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	leaver := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})
	stayer := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(leaver, t, "session:7")
	subscribe(stayer, t, "session:7")
	leaver.waitFrame(t, TypeSubscribed)
	stayer.waitFrame(t, TypeSubscribed)

	// Sanity: presencia de stayer le llega a leaver mientras está suscripto.
	stayer.send(t, map[string]any{"type": TypePresence, "payload": map[string]any{"n": 1}})
	leaver.waitFrame(t, TypePresence)

	leaver.send(t, map[string]any{"type": TypeUnsubscribe, "channel": "session:7"})
	require.Eventually(t, func() bool { return server.hub.Count("session:7") == 1 }, 2*time.Second, 20*time.Millisecond, "quedó solo stayer")

	stayer.send(t, map[string]any{"type": TypePresence, "payload": map[string]any{"n": 2}})
	leaver.waitSilence(t, 300*time.Millisecond)
	assert.Equal(t, 1, server.hub.Count("session:7"))
}

func TestOversizeFrameClosesConnection(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	client.sendRaw(t, []byte(`{"type":"presence","payload":{"blob":"`+strings.Repeat("x", maxMessageSize+1)+`"}}`))
	client.expectDisconnect(t, 2*time.Second)
}

func TestChannelLimitKeepsAliveAndKeepsExisting(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, lenientAuthorizer{})
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	for i := 1; i <= maxChannelsPerConn; i++ {
		channel := fmt.Sprintf("session:%d", 100+i)
		client.send(t, map[string]any{"type": TypeSubscribe, "channel": channel})
		assert.Equal(t, channel, client.waitFrame(t, TypeSubscribed).Channel, "canal %q", channel)
	}

	// El canal 21: error, conexión viva, y los 20 siguen suscriptos.
	subscribe(client, t, "session:999")
	assert.Contains(t, client.waitFrame(t, TypeError).Message, "tope")
	assert.Equal(t, 0, server.hub.Count("session:999"))

	// Resub a un canal PROPIO con el tope lleno: `subscribed` idempotente,
	// el tope se evalúa después de la dedup.
	subscribe(client, t, "session:101")
	assert.Equal(t, "session:101", client.waitFrame(t, TypeSubscribed).Channel)
	assert.Equal(t, 1, server.hub.Count("session:101"))

	// Resub a un canal AJENO con el tope lleno: error, conexión viva.
	subscribe(client, t, "session:888")
	assert.Contains(t, client.waitFrame(t, TypeError).Message, "tope")

	// Cada canal deja de tener exactamente 1 suscriptor (sin perder los primeros).
	assert.Equal(t, 1, server.hub.Count("session:101"))
	assert.Equal(t, 1, server.hub.Count("session:120"))

	pingPong(t, client)
}

func TestDisconnectCleansSubscriptions(t *testing.T) {
	setGatewayJWTConfig(t)
	server := newWSUpgradeServer(t, fakeFullAuthorizer(1))
	client := dialWS(t, server.wsURL+"?token="+tok(t, 1), http.Header{})

	subscribe(client, t, "session:7")
	client.waitFrame(t, TypeSubscribed)
	require.Equal(t, 1, server.hub.Count("session:7"))

	// Cerrar la conexión limpia sus suscripciones.
	require.NoError(t, client.conn.Close())
	require.Eventually(t, func() bool { return server.hub.Count("session:7") == 0 }, 2*time.Second, 20*time.Millisecond)
}
