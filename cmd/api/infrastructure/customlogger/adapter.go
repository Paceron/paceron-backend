package customlogger

import (
	"context"
	"fmt"

	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

type LoggerAdapter struct{}

func NewHTTPClientLogger() httpclient.Logger {
	return &LoggerAdapter{}
}

func (a *LoggerAdapter) Info(ctx context.Context, message string, fields map[string]any) {
	Info(nil, message, parseFields(fields)...)
}

func (a *LoggerAdapter) Warn(ctx context.Context, message string, fields map[string]any) {
	Warn(nil, message, parseFields(fields)...)
}

func (a *LoggerAdapter) Error(ctx context.Context, message string, err error, fields map[string]any) {
	Error(nil, message, err, parseFields(fields)...)
}

func parseFields(fields map[string]any) []string {
	tags := make([]string, 0, len(fields))
	for key, value := range fields {
		tags = append(tags, fmt.Sprintf("%s:%v", key, value))
	}
	return tags
}
