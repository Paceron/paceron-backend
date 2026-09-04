package controllers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type PingController interface {
	Ping(c *gin.Context)
	CallbackTkn(c *gin.Context)
}

type pingController struct{}

func NewPingController() PingController {
	return &pingController{}
}

// Ping godoc
// @Summary      Health check
// @Description  Returns pong if the server is running
// @Tags         health
// @Success      200  {string}  string  "pong"
// @Router       /ping [get]
func (p *pingController) Ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

// CallbackTkn godoc
// @Summary      Debug OAuth callback
// @Description  Loguea todos los query params recibidos (util para debug de OAuth callback)
// @Tags         health
// @Success      200  {string}  string  "ok"
// @Router       /callbackauth [get]
func (p *pingController) CallbackTkn(c *gin.Context) {
	query := c.Request.URL.Query()
	customlogger.Info(c, "callbackauth raw query", customlogger.Tag("raw", c.Request.URL.RawQuery))

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		customlogger.Info(c, "callbackauth param", customlogger.Tag("param", k), customlogger.Tag("value", strings.Join(query[k], ",")))
	}

	c.String(http.StatusOK, "ok")
}
