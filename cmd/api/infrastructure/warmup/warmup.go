package warmup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

const (
	requestTimeout  = 10 * time.Second
	maxResponseBody = 4 * 1024
)

// Run consulta /ping inmediatamente y luego en cada intervalo hasta que ctx se
// cancele. Debe ejecutarse en una goroutine para no bloquear el servidor.
func Run(ctx context.Context, cfg config.WarmupConfig) {
	run(ctx, cfg, &http.Client{Timeout: requestTimeout})
}

func run(ctx context.Context, cfg config.WarmupConfig, client *http.Client) {
	if cfg.BaseURL == "" || cfg.Interval <= 0 {
		return
	}

	targetURL := cfg.BaseURL + "/ping"
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	ping(ctx, client, targetURL)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping(ctx, client, targetURL)
		}
	}
}

func ping(ctx context.Context, client *http.Client, targetURL string) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		customlogger.Warn(nil, "warmup request could not be created",
			customlogger.Tag("url", targetURL),
			customlogger.Tag("error", err.Error()))
		return
	}

	response, err := client.Do(request)
	if err != nil {
		customlogger.Warn(nil, "warmup request failed",
			customlogger.Tag("url", targetURL),
			customlogger.Tag("error", err.Error()))
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBody))

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		customlogger.Warn(nil, "warmup returned a non-success status",
			customlogger.Tag("url", targetURL),
			customlogger.Tag("status_code", fmt.Sprintf("%d", response.StatusCode)))
		return
	}
	customlogger.Debug(nil, "warmup request succeeded",
		customlogger.Tag("url", targetURL),
		customlogger.Tag("status_code", fmt.Sprintf("%d", response.StatusCode)))
}
