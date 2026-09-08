package customlogger

import (
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

func TestGetFieldsKeepsColonsInValue(t *testing.T) {
	fields, err := getFields([]string{"url:/api/v1/payments/:id", "method:ProcessPayment"})
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/payments/:id", fields["url"])
	assert.Equal(t, "ProcessPayment", fields["method"])
}