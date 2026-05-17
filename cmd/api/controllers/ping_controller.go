package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PingController interface {
	Ping(c *gin.Context)
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
