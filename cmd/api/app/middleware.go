package app

import (
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
