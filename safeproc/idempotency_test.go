package safeproc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIdempotencyReplay(t *testing.T) {
	var calls int
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		WriteJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	}), IdempotencyKey(IdempotencyConfig{TTL: time.Minute, MaxBodyBytes: 1024}))

	body := `{"x":1}`
	r1 := httptest.NewRequest("POST", "/v1/events", strings.NewReader(body))
	r1.Header.Set("Idempotency-Key", "abc")
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)

	r2 := httptest.NewRequest("POST", "/v1/events", strings.NewReader(body))
	r2.Header.Set("Idempotency-Key", "abc")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)

	if calls != 1 {
		t.Fatalf("expected handler to execute once, got %d", calls)
	}
	if got := w2.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected replay header=true, got %q", got)
	}
}

func TestIdempotencyConflict(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), IdempotencyKey(IdempotencyConfig{TTL: time.Minute, MaxBodyBytes: 1024}))

	r1 := httptest.NewRequest("POST", "/v1/events", strings.NewReader(`{"x":1}`))
	r1.Header.Set("Idempotency-Key", "same")
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)

	r2 := httptest.NewRequest("POST", "/v1/events", strings.NewReader(`{"x":2}`))
	r2.Header.Set("Idempotency-Key", "same")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestIdempotencyDifferentPath(t *testing.T) {
	var calls int
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}), IdempotencyKey(IdempotencyConfig{TTL: time.Minute, MaxBodyBytes: 1024}))

	for i, path := range []string{"/a", "/b"} {
		r := httptest.NewRequest("POST", path, strings.NewReader(`{"x":1}`))
		r.Header.Set("Idempotency-Key", "same")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("request %d unexpected code: %d", i, w.Code)
		}
	}
	if calls != 2 {
		t.Fatalf("expected two real calls, got %d", calls)
	}
}

func TestIdempotencyTooLarge(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), IdempotencyKey(IdempotencyConfig{TTL: time.Minute, MaxBodyBytes: 2}))

	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"x":1}`))
	r.Header.Set("Idempotency-Key", "k")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d (%s)", w.Code, fmt.Sprintf("%s", w.Body.String()))
	}
}
