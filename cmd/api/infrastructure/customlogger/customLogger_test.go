package customlogger

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddURLToTagsRequiresFullPath(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	u, err := url.Parse("/api/v1/payments/1351561837")
	require.NoError(t, err)
	ctx.Request = &http.Request{Method: http.MethodGet, URL: u}

	// Sin gin engine, FullPath() no está disponible vía route; el ctx queda sin
	// ruta registrada, así que el helper no agrega nada (a propósito, no loguea
	// el path real con IDs).
	tags := []string{}
	addURLToTags(ctx, &tags)
	assert.Empty(t, tags)
}

func TestAddURLToTagsNilContext(t *testing.T) {
	tags := []string{}
	addURLToTags(nil, &tags)
	assert.Empty(t, tags)
}

func TestAddRequestIDToTagsNilWriterDoesNotPanic(t *testing.T) {
	ctx := &gin.Context{}

	tags := []string{}
	require.NotPanics(t, func() { addRequestIDToTags(ctx, &tags) })
	assert.Empty(t, tags)
}

// TestAddRequestIDToTagsFromCopiedContext reproduce el log async: ctx.Copy()
// deja ResponseWriter nil y el helper no debe tocar ctx.Writer.Header().
func TestAddRequestIDToTagsFromCopiedContext(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	ctx.Set("X-Request-Id", "req-from-keys")
	copied := ctx.Copy()

	tags := []string{}
	require.NotPanics(t, func() { addRequestIDToTags(copied, &tags) })
	assert.Contains(t, tags, _XrequestID+":req-from-keys")
}

func TestAddRequestIDToTagsFallsBackToRequestHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	request.Header.Set("X-Request-Id", "req-from-header")
	ctx.Request = request
	copied := ctx.Copy()

	tags := []string{}
	require.NotPanics(t, func() { addRequestIDToTags(copied, &tags) })
	assert.Contains(t, tags, _XrequestID+":req-from-header")
}

func TestErrorOnCopiedContextDoesNotPanic(t *testing.T) {
	previousShowRequestID := TLogger.showRequestID
	previousOut := TLogger.log.Out
	TLogger.showRequestID = true
	TLogger.log.Out = io.Discard
	t.Cleanup(func() {
		TLogger.showRequestID = previousShowRequestID
		TLogger.log.Out = previousOut
	})

	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	ctx.Set("X-Request-Id", "req-error")
	copied := ctx.Copy()

	require.NotPanics(t, func() { Error(copied, "boom", errors.New("fallo")) })
}

func TestGetFieldsKeepsColonsInValue(t *testing.T) {
	fields, err := getFields([]string{"url:/api/v1/payments/:id", "method:ProcessPayment"})
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/payments/:id", fields["url"])
	assert.Equal(t, "ProcessPayment", fields["method"])
}
