package safeproc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

type Processor struct {
	maxBodyBytes int64
}

type Config struct {
	MaxBodyBytes int64
}

type Validator[T any] func(v T) error

type Sanitizer[T any] func(v *T)

func NewProcessor(cfg Config) *Processor {
	maxBytes := cfg.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBodyBytes
	}
	return &Processor{maxBodyBytes: maxBytes}
}

func (p *Processor) DecodeJSON(r *http.Request, dst any) *HTTPError {
	if r.Body == nil {
		return ErrEmptyBody
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, p.maxBodyBytes+1))
	if err != nil {
		return ErrInvalidJSON
	}
	if int64(len(body)) > p.maxBodyBytes {
		return ErrPayloadTooLarge
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return ErrEmptyBody
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return ErrUnknownField
		}
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
			return ErrInvalidJSON
		}
		return ErrInvalidJSON
	}

	// Ensure there is only one JSON value in request body.
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidJSON
	}
	return nil
}

func ParseAndValidate[T any](p *Processor, r *http.Request, validate Validator[T], sanitize Sanitizer[T]) (T, *HTTPError) {
	var in T
	if err := p.DecodeJSON(r, &in); err != nil {
		return in, err
	}
	if sanitize != nil {
		sanitize(&in)
	}
	if validate != nil {
		if err := validate(in); err != nil {
			return in, newErr(ErrValidation.Status, ErrValidation.Code, fmt.Sprintf("%s: %v", ErrValidation.Message, err))
		}
	}
	return in, nil
}
