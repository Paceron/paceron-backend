package app

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
)

const (
	_XrequestID  = "X-Request-Id"
	_Flow        = "Flow"
	_StringEmpty = ""
)

func SetRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil {
			rqID := c.Request.Header.Get(_XrequestID)
			if rqID == _StringEmpty {
				uuid4, _ := uuid.NewV4()
				rqID = uuid4.String()
			}
			c.Set(_XrequestID, rqID)
			c.Writer.Header().Set(_XrequestID, rqID)
			c.Next()
		}
	}
}

func SetFlow() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil {
			flow := c.Request.URL.Path
			c.Set(_Flow, flow)
			c.Writer.Header().Set(_Flow, flow)
			c.Next()
		}
	}
}

func CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	var origins []string
	if allowedOrigins != "" {
		for _, o := range strings.Split(allowedOrigins, ",") {
			origins = append(origins, strings.TrimSpace(o))
		}
	} else {
		origins = []string{
			"http://localhost:8081",
			"http://localhost:5173",
			"http://localhost:3000",
		}
	}

	originMap := make(map[string]bool, len(origins))
	for _, o := range origins {
		originMap[o] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if originMap[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-Id, X-Current-Password")
			c.Header("Access-Control-Max-Age", "86400")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
