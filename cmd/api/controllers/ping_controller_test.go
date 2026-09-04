package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/ping", nil)

	controller := NewPingController()
	controller.Ping(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "pong", response.Body.String())
}

func TestCallbackTkn(t *testing.T) {
	t.Run("con query params", func(t *testing.T) {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request, _ = http.NewRequest(http.MethodGet, "/callbackauth?code=CODE_123&state=7-918273645", nil)

		controller := NewPingController()
		controller.CallbackTkn(c)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "ok", response.Body.String())
	})

	t.Run("sin query params", func(t *testing.T) {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request, _ = http.NewRequest(http.MethodGet, "/callbackauth", nil)

		controller := NewPingController()
		controller.CallbackTkn(c)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "ok", response.Body.String())
	})
}
