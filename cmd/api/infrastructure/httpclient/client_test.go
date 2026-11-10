package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("lectura rota") }
func (failingReader) Close() error             { return nil }

type recordingLogger struct {
	info, warn, err int
}

func (l *recordingLogger) Info(context.Context, string, map[string]any)         { l.info++ }
func (l *recordingLogger) Warn(context.Context, string, map[string]any)         { l.warn++ }
func (l *recordingLogger) Error(context.Context, string, error, map[string]any) { l.err++ }

func TestClient_MethodsEncodeYDecodean(t *testing.T) {
	var gotMethod, gotHeader, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotHeader, gotPath = r.Method, r.Header.Get("X-Test"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client := New(WithBaseURL(server.URL), WithHeader("X-Test", "si"))
	var response struct {
		OK bool `json:"ok"`
	}

	require.NoError(t, client.Get(context.Background(), "/get", &response))
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "si", gotHeader)
	assert.Equal(t, "/get", gotPath)
	assert.True(t, response.OK)

	require.NoError(t, client.Post(context.Background(), "/post", map[string]string{"a": "b"}, &response))
	assert.Equal(t, http.MethodPost, gotMethod)

	require.NoError(t, client.Put(context.Background(), "/put", map[string]string{"a": "b"}, &response))
	assert.Equal(t, http.MethodPut, gotMethod)

	require.NoError(t, client.Delete(context.Background(), "/delete", &response))
	assert.Equal(t, http.MethodDelete, gotMethod)
}

func TestClient_ErrorDeClienteNoReintenta(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("sin recurso"))
	}))
	t.Cleanup(server.Close)

	logger := &recordingLogger{}
	client := New(WithBaseURL(server.URL), WithLogger(logger), WithRetry(2, time.Millisecond))

	err := client.Get(context.Background(), "/missing", nil)

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
	assert.Equal(t, "sin recurso", httpErr.Body)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "un 4xx no se reintenta")
	assert.Equal(t, 1, logger.warn)
}

func TestClient_ErrorDeServidorReintentaYTelemetriza(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("caido"))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	logger := &recordingLogger{}
	client := New(
		WithBaseURL(server.URL),
		WithLogger(logger),
		WithRetry(1, time.Millisecond),
	)
	var response struct {
		OK bool `json:"ok"`
	}

	require.NoError(t, client.Get(context.Background(), "/retry", &response))
	assert.True(t, response.OK)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	assert.Equal(t, 1, logger.warn, "el 5xx intermedio se loguea como warning")
	assert.GreaterOrEqual(t, logger.info, 2, "retry y éxito dejan trazas de info")
}

func TestClient_ErrorDeRedReintentaYDevuelveUltimoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	logger := &recordingLogger{}
	breaker := NewCircuitBreaker(5, time.Hour)
	client := New(
		WithBaseURL(url),
		WithLogger(logger),
		WithRetry(1, time.Millisecond),
		WithCircuitBreaker(breaker),
		WithTelemetry(func(context.Context, TelemetryData) {}),
	)

	err := client.Get(context.Background(), "/down", nil)

	require.Error(t, err)
	assert.Equal(t, 2, logger.err, "cada intento fallido de red loguea error")
}

func TestClient_BreakerAbiertoCortaElRequest(t *testing.T) {
	breaker := NewCircuitBreaker(1, time.Hour)
	breaker.Fail()
	client := New(WithBaseURL("http://example.com"), WithCircuitBreaker(breaker))

	err := client.Get(context.Background(), "/", nil)

	require.ErrorIs(t, err, ErrCircuitBreakerOpen)
}

func TestClient_ErroresDeArmadoYLectura(t *testing.T) {
	t.Run("body no serializable", func(t *testing.T) {
		client := New(WithBaseURL("http://example.com"))
		require.Error(t, client.Post(context.Background(), "/", make(chan int), nil))
	})

	t.Run("url inválida", func(t *testing.T) {
		client := New(WithBaseURL("http://example.com"))
		require.Error(t, client.Get(context.Background(), "/con\nsalto", nil))
	})

	t.Run("respuesta json inválida", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("no-json"))
		}))
		t.Cleanup(server.Close)
		logger := &recordingLogger{}
		client := New(WithBaseURL(server.URL), WithLogger(logger))
		var response struct{ OK bool }
		require.Error(t, client.Get(context.Background(), "/", &response))
		assert.Equal(t, 1, logger.err)
	})

	t.Run("lectura del body falla", func(t *testing.T) {
		client := New(WithBaseURL("http://example.com"), WithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: failingReader{}}, nil
		})))
		require.Error(t, client.Get(context.Background(), "/", nil))
	})
}

func TestClient_GetOutNilLoggerNoRompe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := New(WithBaseURL(server.URL))
	err := client.Get(context.Background(), "/", nil)
	require.Error(t, err)
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestHTTPError_ErrorIncluyeStatusYBody(t *testing.T) {
	err := (&HTTPError{StatusCode: 418, Body: "teapot"}).Error()
	assert.Contains(t, err, "418")
	assert.Contains(t, err, "teapot")
}

func TestClient_ResponseBodyVacioNoIntentaDecodificar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	var response struct{ OK bool }
	require.NoError(t, New(WithBaseURL(server.URL)).Get(context.Background(), "/", &response))
	assert.False(t, response.OK)
}

var _ io.Reader = failingReader{}
