package httpclient

import (
	"context"
	"time"
)

type TelemetryData struct {
	Method       string
	URL          string
	StatusCode   int
	Duration     time.Duration
	Success      bool
	RequestSize  int
	ResponseSize int
	Error        error
}

type TelemetryFunc func(
	ctx context.Context,
	data TelemetryData,
)
