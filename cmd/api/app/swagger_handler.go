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

func serveSwaggerJSON(c *gin.Context) {
	data, err := os.ReadFile("cmd/api/docs/swagger.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "swagger spec not found"})
		return
	}

	env := c.DefaultQuery("env", "local")

	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	switch env {
	case "production":
		spec["host"] = "paceron-backend.onrender.com"
	default:
		spec["host"] = "localhost:8080"
	}

	modified, _ := json.MarshalIndent(spec, "", "    ")
	c.Data(http.StatusOK, "application/json; charset=utf-8", modified)
}


