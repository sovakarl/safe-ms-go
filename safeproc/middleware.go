package safeproc

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func Recover(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.Printf("panic recovered: %v", rec)
					}
					WriteError(w, ErrInternal, logger)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			next.ServeHTTP(w, r)
		})
	}
}

func CORS(allowOrigin string) Middleware {
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-Signature, X-Internal-Token, Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = genRequestID()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := WithRequestID(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func HMACAuth(secret []byte, maxBodyBytes int64) Middleware {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig := r.Header.Get("X-Signature")
			if sig == "" {
				WriteError(w, ErrUnauthorized, nil)
				return
			}

			body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
			if err != nil || int64(len(body)) > maxBodyBytes {
				WriteError(w, ErrUnauthorized, nil)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			expected, err := parseSignature(sig)
			if err != nil {
				WriteError(w, ErrUnauthorized, nil)
				return
			}

			mac := hmac.New(sha256.New, secret)
			_, _ = mac.Write(body)
			sum := mac.Sum(nil)
			if subtle.ConstantTimeCompare(sum, expected) != 1 {
				WriteError(w, ErrUnauthorized, nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func StaticTokenAuth(headerName, token string) Middleware {
	if headerName == "" {
		headerName = "X-Internal-Token"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			given := r.Header.Get(headerName)
			if len(token) == 0 || subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
				WriteError(w, ErrUnauthorized, nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseSignature(v string) ([]byte, error) {
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return nil, fmt.Errorf("invalid signature format")
	}
	decoded, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	if len(decoded) != sha256.Size {
		return nil, fmt.Errorf("invalid signature size")
	}
	return decoded, nil
}

func genRequestID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		// Safe fallback with deterministic value only if CSPRNG failed.
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}
