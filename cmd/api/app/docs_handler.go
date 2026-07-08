package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/utils"
)

const docsDir = "docs/guide"

func mapGuide(r *gin.Engine) {
	r.GET("/api/v1/docs/guide/*any", handleGuide)
}

func handleGuide(c *gin.Context) {
	anyPath := c.Param("any")

	if strings.HasPrefix(anyPath, "/plantuml/") {
		plantUMLProxy(c, strings.TrimPrefix(anyPath, "/plantuml/"))
		return
	}

	serveGuide(c, anyPath)
}

func serveGuide(c *gin.Context, filePath string) {
	if filePath == "" || filePath == "/" {
		filePath = "/index.html"
	}

	fullPath := filepath.Join(docsDir, filePath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.File(fullPath)
}

func plantUMLProxy(c *gin.Context, pumlFile string) {
	fullPath := filepath.Join(docsDir, "diagrams", pumlFile)

	source, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Diagram not found"})
		return
	}

	encoded := utils.EncodePlantUML(string(source))

	plantUMLURL := fmt.Sprintf("https://www.plantuml.com/plantuml/svg/~h%s", encoded)

	c.Redirect(http.StatusFound, plantUMLURL)
}
