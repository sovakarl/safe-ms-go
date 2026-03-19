package safeproc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), RateLimit(RateLimitConfig{RatePerSecond: 1, Burst: 2, EntryTTL: time.Minute}))

	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("GET", "/", strings.NewReader(""))
		r.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("request %d expected 204, got %d", i, w.Code)
		}
	}

	r := httptest.NewRequest("GET", "/", strings.NewReader(""))
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}
