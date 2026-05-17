package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSetRequestId(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: make(http.Header)}

	SetRequestID()(ctx)

	requestID, exists := ctx.Get(_XrequestID)
	assert.True(t, exists)
	assert.NotEmpty(t, requestID)
}

func TestSetRequestId_ExistingHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{_XrequestID: []string{"existing-id"}}}

	SetRequestID()(ctx)

	requestID, _ := ctx.Get(_XrequestID)
	assert.Equal(t, "existing-id", requestID)
}
