package safeproc

import "net/http"

// HTTPError is a safe, client-facing error.
type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

func newErr(status int, code, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}

var (
	ErrInvalidJSON     = newErr(http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
	ErrEmptyBody       = newErr(http.StatusBadRequest, "empty_body", "Request body is empty")
	ErrUnknownField    = newErr(http.StatusBadRequest, "unknown_field", "Payload contains unknown field")
	ErrValidation      = newErr(http.StatusUnprocessableEntity, "validation_failed", "Payload validation failed")
	ErrPayloadTooLarge = newErr(http.StatusRequestEntityTooLarge, "payload_too_large", "Payload is too large")
	ErrUnauthorized    = newErr(http.StatusUnauthorized, "unauthorized", "Request authentication failed")
	ErrForbidden       = newErr(http.StatusForbidden, "forbidden", "Access denied")
	ErrConflict        = newErr(http.StatusConflict, "conflict", "Request conflicts with existing operation")
	ErrTooManyRequests = newErr(http.StatusTooManyRequests, "too_many_requests", "Too many requests")
	ErrUpstream        = newErr(http.StatusBadGateway, "upstream_error", "Upstream service error")
	ErrNotFound        = newErr(http.StatusNotFound, "not_found", "Resource not found")
	ErrInternal        = newErr(http.StatusInternalServerError, "internal_error", "Internal server error")
)

func WithMessage(base *HTTPError, message string) *HTTPError {
	if base == nil {
		return nil
	}
	return &HTTPError{Status: base.Status, Code: base.Code, Message: message}
}
