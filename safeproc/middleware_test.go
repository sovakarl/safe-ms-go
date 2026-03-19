package safeproc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sign(secret []byte, body string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestHMACAuth_OK(t *testing.T) {
	secret := []byte("test-secret")
	body := `{"a":1}`

	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), HMACAuth(secret, 1024))

	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("X-Signature", sign(secret, body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHMACAuth_Fail(t *testing.T) {
	secret := []byte("test-secret")
	body := `{"a":1}`

	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), HMACAuth(secret, 1024))

	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("X-Signature", fmt.Sprintf("sha256=%064s", "bad"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
