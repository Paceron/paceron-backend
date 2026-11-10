package customlogger

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withLoggerState(t *testing.T, level logrus.Level) {
	t.Helper()
	previousLevel := TLogger.log.Level
	previousOut := TLogger.log.Out
	previousShowURL := TLogger.showURL
	TLogger.log.Level = level
	TLogger.log.Out = io.Discard
	t.Cleanup(func() {
		TLogger.log.Level = previousLevel
		TLogger.log.Out = previousOut
		TLogger.showURL = previousShowURL
	})
}

func TestLoggerWrappers_NoRompenSinContexto(t *testing.T) {
	withLoggerState(t, logrus.DebugLevel)

	require.NotPanics(t, func() {
		Print("print")
		Printf("printf %d", 1)
		Debugf(nil, "debug %d", 1)
		Infof(nil, "info %d", 1)
		Warnf(nil, "warn %d", 1)
		Errorf(nil, "error %d", errors.New("boom"), 1)
	})
}

func TestSetShowURLYGetOut(t *testing.T) {
	withLoggerState(t, logrus.InfoLevel)

	SetShowURL(true)
	assert.True(t, TLogger.showURL)
	SetShowURL(false)
	assert.False(t, TLogger.showURL)
	assert.NotNil(t, GetOut())
}

func TestSetLogLevel_ValidoEInvalido(t *testing.T) {
	withLoggerState(t, logrus.InfoLevel)

	setLogLevel(InfoLevel)
	assert.Equal(t, logrus.InfoLevel, TLogger.log.Level)

	require.Panics(t, func() { setLogLevel("nivel-inexistente") })
}

func TestPanicf_LogueaYPropagaElPanic(t *testing.T) {
	withLoggerState(t, logrus.PanicLevel)

	require.Panics(t, func() { Panicf(nil, "panic %d", errors.New("boom"), 1) })
}

func TestAddRequestIDToTagsNilContext(t *testing.T) {
	tags := []string{"a:b"}
	require.NotPanics(t, func() { addRequestIDToTags(nil, &tags) })
	assert.Equal(t, []string{"a:b"}, tags)
}

func TestAddURLToTagsConRutaRegistrada(t *testing.T) {
	withLoggerState(t, logrus.DebugLevel)
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	var tags []string
	engine.GET("/api/v1/payments/:id", func(ctx *gin.Context) {
		tags = []string{}
		addURLToTags(ctx, &tags)
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/payments/42", nil))

	assert.Equal(t, []string{_URLTag + ":/api/v1/payments/:id"}, tags)
}

func TestBuildLogEntryConTagInvalidoYShowURL(t *testing.T) {
	withLoggerState(t, logrus.DebugLevel)
	TLogger.showURL = true

	entry, message := buildLogEntry(nil, []string{"sincolon"}, "mensaje")

	require.NotNil(t, entry)
	assert.Contains(t, message, "error parsing tags")
}

func TestHTTPClientLoggerAdapter(t *testing.T) {
	withLoggerState(t, logrus.DebugLevel)
	adapter := NewHTTPClientLogger()

	require.NotPanics(t, func() {
		adapter.Info(context.Background(), "info", map[string]any{"k": "v"})
		adapter.Warn(context.Background(), "warn", map[string]any{"k": 1})
		adapter.Error(context.Background(), "error", errors.New("boom"), map[string]any{"k": true})
	})
	assert.Equal(t, []string{"k:v"}, parseFields(map[string]any{"k": "v"}))
}
