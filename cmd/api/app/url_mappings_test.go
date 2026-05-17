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
	assert.True(t, routes[http.MethodGet+":"+"/user/:user_id"], "GET /user/:user_id route should exist")
	assert.True(t, routes[http.MethodPost+":"+"/user"], "POST /user route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/example/weather"], "GET /example/weather route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/user/:user_id/weather"], "GET /user/:user_id/weather route should exist")
	assert.True(t, routes[http.MethodGet+":"+"/swagger/*any"], "GET /swagger/*any route should exist")
}
