// ================================
// internal/httpclient/errors.go
// ================================

package httpclient

import "fmt"

type HTTPError struct {
	StatusCode int
	Body       string
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf(
		"http request failed status=%d code=%s message=%s body=%s",
		e.StatusCode,
		e.Code,
		e.Message,
		e.Body,
	)
}
