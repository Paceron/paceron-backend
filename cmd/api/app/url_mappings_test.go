package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPingRouteExists(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	app := NewApplication()
	mapUrls(router, app)

	routes := make(map[string]bool)
	for _, r := range router.Routes() {
		routes[r.Method+":"+r.Path] = true
	}

	assert.True(t, routes[http.MethodGet+":"+"/ping"], "GET /ping route should exist")
	assert.False(t, routes[http.MethodGet+":"+"/user/:user_id"], "legacy GET /user/:user_id route should have been removed")
	assert.False(t, routes[http.MethodPost+":"+"/user"], "legacy POST /user route should have been removed")
	assert.True(t, routes[http.MethodGet+":"+"/example/weather"], "GET /example/weather route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/user/:user_id/weather"], "GET /user/:user_id/weather route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/api/v1/ws"], "GET /api/v1/ws route should exist (gateway WS público)")
	assert.False(t, routes[http.MethodGet+":"+"/api/v1/sessions/:id/assigned-groups"], "assigned-groups route should have been removed")
}

// TestWSRouteIsPublic prueba el orden del wiring: /api/v1/ws está ANTES del
// r.Use(AuthMiddleware()), y su propio handler responde el 401 con el mensaje
// del query param (distinto del de AuthMiddleware "falta el header ...").
func TestWSRouteIsPublic(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	app := NewApplication()
	mapUrls(router, app)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "falta el query param token")
	assert.NotContains(t, w.Body.String(), "falta el header Authorization")
}
