package httputils

import "net/http"

func GetStatusCode(status string) int {
	switch status {
	case "ok":
		return http.StatusOK
	case "created":
		return http.StatusCreated
	case "not_found":
		return http.StatusNotFound
	case "bad_request":
		return http.StatusBadRequest
	case "internal_error":
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
