package safeproc

import (
	"net/http/httptest"
	"strings"
	"testing"
)

type payload struct {
	Name string `json:"name"`
}

func TestDecodeJSON_StrictUnknownField(t *testing.T) {
	p := NewProcessor(Config{MaxBodyBytes: 1024})
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a","x":1}`))

	var got payload
	err := p.DecodeJSON(r, &got)
	if err == nil || err.Code != ErrUnknownField.Code {
		t.Fatalf("expected unknown field error, got %#v", err)
	}
}

func TestDecodeJSON_TooLarge(t *testing.T) {
	p := NewProcessor(Config{MaxBodyBytes: 5})
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"abc"}`))

	var got payload
	err := p.DecodeJSON(r, &got)
	if err == nil || err.Code != ErrPayloadTooLarge.Code {
		t.Fatalf("expected payload too large error, got %#v", err)
	}
}

func TestParseAndValidate(t *testing.T) {
	p := NewProcessor(Config{MaxBodyBytes: 1024})
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"  ok  "}`))

	v, err := ParseAndValidate(p, r,
		func(v payload) error { return nil },
		func(v *payload) { v.Name = strings.TrimSpace(v.Name) },
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v.Name != "ok" {
		t.Fatalf("expected sanitized value, got %q", v.Name)
	}
}
