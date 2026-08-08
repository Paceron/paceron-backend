package app

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"

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
	r.GET("/swagger/doc.json", serveSwaggerJSON)
	r.GET("/swagger/swagger.json", serveSwaggerJSON)
}

// serveSwaggerJSON reescribe el "host" del spec con el host real del request
// entrante (c.Request.Host), en vez de una lista fija de entornos conocidos —
// así funciona igual en local, cualquier deploy de Render (producción, staging,
// futuros), o cualquier otro dominio, sin tener que agregar cada uno a mano acá.
func serveSwaggerJSON(c *gin.Context) {
	data, err := os.ReadFile("cmd/api/docs/swagger.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "swagger spec not found"})
		return
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	spec["host"] = c.Request.Host

	modified, _ := json.MarshalIndent(spec, "", "    ")
	c.Data(http.StatusOK, "application/json; charset=utf-8", modified)
}


