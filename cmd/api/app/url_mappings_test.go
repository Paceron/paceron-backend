package app

import (
	"net/http"
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
	assert.True(t, routes[http.MethodGet+":"+"/swagger/index.html"], "GET /swagger/index.html route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/swagger/doc.json"], "GET /swagger/doc.json route should exist")
}
