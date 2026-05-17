// =================================
// internal/httpclient/options.go
// =================================

package httpclient

import (
	"net/http"
	"time"
)

type Option func(*Client)

func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = timeout
	}
}

func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.headers[key] = value
	}
}

func WithLogger(logger Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

func WithTelemetry(fn TelemetryFunc) Option {
	return func(c *Client) {
		c.telemetry = fn
	}
}

func WithCircuitBreaker(cb *CircuitBreaker) Option {
	return func(c *Client) {
		c.breaker = cb
	}
}

func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) {
		c.http.Transport = transport
	}
}

func WithRetry(maxRetries int, retryDelay time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryDelay = retryDelay
	}
}
