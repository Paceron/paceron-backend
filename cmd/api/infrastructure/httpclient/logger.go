package httpclient

import "context"

type Logger interface {
	Info(
		ctx context.Context,
		message string,
		fields map[string]any,
	)

	Warn(
		ctx context.Context,
		message string,
		fields map[string]any,
	)

	Error(
		ctx context.Context,
		message string,
		err error,
		fields map[string]any,
	)
}
