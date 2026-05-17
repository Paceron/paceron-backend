package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	headers map[string]string

	http *http.Client

	logger     Logger
	telemetry  TelemetryFunc
	breaker    *CircuitBreaker
	maxRetries int
	retryDelay time.Duration
}

func New(opts ...Option) *Client {

	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := &Client{
		headers: make(map[string]string),
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) Get(
	ctx context.Context,
	path string,
	response any,
) error {
	return c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		response,
	)
}

func (c *Client) Post(
	ctx context.Context,
	path string,
	body any,
	response any,
) error {
	return c.do(
		ctx,
		http.MethodPost,
		path,
		body,
		response,
	)
}

func (c *Client) Put(
	ctx context.Context,
	path string,
	body any,
	response any,
) error {
	return c.do(
		ctx,
		http.MethodPut,
		path,
		body,
		response,
	)
}

func (c *Client) Delete(
	ctx context.Context,
	path string,
	response any,
) error {
	return c.do(
		ctx,
		http.MethodDelete,
		path,
		nil,
		response,
	)
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	requestBody any,
	responseBody any,
) error {

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {

		if attempt > 0 {
			c.logInfo(ctx, "retrying request", map[string]any{
				"method":  method,
				"path":    path,
				"attempt": attempt,
				"max":     c.maxRetries,
			})
			time.Sleep(c.retryDelay)
		}

		if c.breaker != nil {
			if err := c.breaker.Allow(); err != nil {
				return err
			}
		}

		start := time.Now()

		var requestBytes []byte
		var bodyReader io.Reader

		if requestBody != nil {
			var err error
			requestBytes, err = json.Marshal(requestBody)
			if err != nil {
				return err
			}
			bodyReader = bytes.NewBuffer(requestBytes)
		}

		url := c.baseURL + path

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		for key, value := range c.headers {
			req.Header.Set(key, value)
		}

		resp, err := c.http.Do(req)

		duration := time.Since(start)

		telemetry := TelemetryData{
			Method:      method,
			URL:         url,
			Duration:    duration,
			RequestSize: len(requestBytes),
			Error:       err,
		}

		if err != nil {
			lastErr = err

			if c.breaker != nil {
				c.breaker.Fail()
			}

			c.logError(ctx, "http request failed", err, map[string]any{
				"method":  method,
				"url":     url,
				"attempt": attempt,
			})

			if c.telemetry != nil {
				c.telemetry(ctx, telemetry)
			}

			if attempt < c.maxRetries {
				continue
			}

			return err
		}

		responseBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			return readErr
		}

		telemetry.StatusCode = resp.StatusCode
		telemetry.ResponseSize = len(responseBytes)

		if resp.StatusCode >= 500 {
			lastErr = &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(responseBytes),
			}

			if c.breaker != nil {
				c.breaker.Fail()
			}

			c.logWarn(ctx, "http request returned server error", map[string]any{
				"method":  method,
				"url":     url,
				"status":  resp.StatusCode,
				"attempt": attempt,
			})

			if attempt < c.maxRetries {
				continue
			}

			return lastErr
		}

		if c.breaker != nil {
			c.breaker.Success()
		}

		if resp.StatusCode >= 400 {
			httpErr := &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(responseBytes),
			}

			c.logWarn(ctx, "http request returned client error", map[string]any{
				"method": method,
				"url":    url,
				"status": resp.StatusCode,
			})

			return httpErr
		}

		if responseBody != nil && len(responseBytes) > 0 {
			if err := json.Unmarshal(responseBytes, responseBody); err != nil {
				c.logError(ctx, "failed to parse response", err, map[string]any{
					"method": method,
					"url":    url,
				})
				return err
			}
		}

		c.logInfo(ctx, "http request completed", map[string]any{
			"method":   method,
			"url":      url,
			"status":   resp.StatusCode,
			"duration": duration.String(),
		})

		return nil
	}

	return lastErr
}

func (c *Client) logInfo(
	ctx context.Context,
	message string,
	fields map[string]any,
) {
	if c.logger == nil {
		return
	}

	c.logger.Info(
		ctx,
		message,
		fields,
	)
}

func (c *Client) logWarn(
	ctx context.Context,
	message string,
	fields map[string]any,
) {
	if c.logger == nil {
		return
	}

	c.logger.Warn(
		ctx,
		message,
		fields,
	)
}

func (c *Client) logError(
	ctx context.Context,
	message string,
	err error,
	fields map[string]any,
) {
	if c.logger == nil {
		return
	}

	c.logger.Error(
		ctx,
		message,
		err,
		fields,
	)
}
