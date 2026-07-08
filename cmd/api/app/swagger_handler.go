package app

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed swagger_custom.html
var customSwaggerHTML string

func mapSwagger(r *gin.Engine) {
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/index.html", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, customSwaggerHTML)
	})
	r.StaticFile("/swagger/doc.json", "cmd/api/docs/swagger.json")
	r.StaticFile("/swagger/swagger.json", "cmd/api/docs/swagger.json")
}
