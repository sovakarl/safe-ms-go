package safeproc

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, err *HTTPError, logger *log.Logger) {
	if err == nil {
		err = ErrInternal
	}
	if logger != nil && err.Status >= http.StatusInternalServerError {
		logger.Printf("internal error: %s", err.Error())
	}
	WriteJSON(w, err.Status, map[string]any{"error": err})
}
