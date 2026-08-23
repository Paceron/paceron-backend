package expopushclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

// TestSend_SendsExpectedRequest levanta un servidor HTTP real y verifica el
// request que arma el cliente contra el shape esperado por la API de Expo — no
// mockea el httpClient, ejercita la construcción real del body.
func TestSend_SendsExpectedRequest(t *testing.T) {
	var receivedPath string
	var receivedBody sendPushRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"status":"ok","id":"receipt-id"}}`))
	}))
	defer server.Close()

	httpClient := httpclient.New(httpclient.WithBaseURL(server.URL))
	client := New(httpClient)

	err := client.Send(context.Background(), "ExponentPushToken[xxx]", "Nueva invitación", "Juan te invitó a un equipo", map[string]string{
		"type":  "invitation_received",
		"route": "/invitations",
	})

	require.NoError(t, err)
	assert.Equal(t, sendPath, receivedPath)
	assert.Equal(t, "ExponentPushToken[xxx]", receivedBody.To)
	assert.Equal(t, "Nueva invitación", receivedBody.Title)
	assert.Equal(t, "Juan te invitó a un equipo", receivedBody.Body)
	assert.Equal(t, "invitation_received", receivedBody.Data["type"])
	assert.Equal(t, "/invitations", receivedBody.Data["route"])
}

// TestSend_WithoutRoute cubre el caso informativo (ej. password_changed) que no
// navega a ningún lado — data.route puede venir ausente.
func TestSend_WithoutRoute(t *testing.T) {
	var receivedBody sendPushRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"status":"ok"}}`))
	}))
	defer server.Close()

	httpClient := httpclient.New(httpclient.WithBaseURL(server.URL))
	client := New(httpClient)

	err := client.Send(context.Background(), "ExponentPushToken[xxx]", "Contraseña cambiada", "Tu contraseña se actualizó correctamente", map[string]string{
		"type": "password_changed",
	})

	require.NoError(t, err)
	_, hasRoute := receivedBody.Data["route"]
	assert.False(t, hasRoute)
}

func TestSend_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	httpClient := httpclient.New(httpclient.WithBaseURL(server.URL), httpclient.WithRetry(0, 0))
	client := New(httpClient)

	err := client.Send(context.Background(), "ExponentPushToken[xxx]", "t", "b", nil)

	assert.Error(t, err)
}

func TestNew_ImplementsInterface(t *testing.T) {
	httpClient := httpclient.New()
	client := New(httpClient)
	var iface ExpoPushClientInterface = client
	assert.NotNil(t, iface)
}
