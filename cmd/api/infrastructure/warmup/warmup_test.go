package warmup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"simple-arq-golang/cmd/api/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRequestsPingImmediatelyAndPeriodically(t *testing.T) {
	requests := make(chan *http.Request, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, config.WarmupConfig{BaseURL: server.URL, Interval: 20 * time.Millisecond}, server.Client())
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	for i := 0; i < 2; i++ {
		select {
		case request := <-requests:
			assert.Equal(t, http.MethodGet, request.Method)
			assert.Equal(t, "/ping", request.URL.Path)
		case <-time.After(time.Second):
			require.FailNow(t, "no se recibió el ping %d", i+1)
		}
	}
}

func TestRunContinuesAfterNonSuccessResponse(t *testing.T) {
	var requestCount atomic.Int32
	requests := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		requests <- struct{}{}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, config.WarmupConfig{BaseURL: server.URL, Interval: 20 * time.Millisecond}, server.Client())
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	for i := 0; i < 2; i++ {
		select {
		case <-requests:
		case <-time.After(time.Second):
			require.FailNow(t, "el worker no realizó el intento %d", i+1)
		}
	}
	assert.EqualValues(t, 2, requestCount.Load())
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, config.WarmupConfig{BaseURL: server.URL, Interval: 20 * time.Millisecond}, server.Client())
	}()

	deadline := time.After(time.Second)
	for requestCount.Load() == 0 {
		select {
		case <-deadline:
			require.FailNow(t, "no se recibió el ping inicial")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.FailNow(t, "el worker no terminó después de cancelar el contexto")
	}
	time.Sleep(40 * time.Millisecond)
	assert.EqualValues(t, 1, requestCount.Load())
}

func TestPingUsesClientTimeout(t *testing.T) {
	requestCancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(requestCancelled)
	}))
	defer server.Close()

	done := make(chan struct{})
	go func() {
		ping(context.Background(), &http.Client{Timeout: 20 * time.Millisecond}, server.URL+"/ping")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.FailNow(t, "la request excedió el timeout del cliente")
	}
	select {
	case <-requestCancelled:
	case <-time.After(time.Second):
		assert.Fail(t, "el timeout no canceló la request HTTP")
	}
}
