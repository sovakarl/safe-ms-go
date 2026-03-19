package safeproc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"
)

type IdempotencyConfig struct {
	TTL            time.Duration
	MaxEntries     int
	MaxBodyBytes   int64
	HeaderName     string
	ReplayHeader   string
	Store          *IdempotencyStore
	TrackedMethods map[string]struct{}
}

type IdempotencyStore struct {
	mu      sync.RWMutex
	items   map[string]idemRecord
	maxSize int
}

type idemRecord struct {
	BodyHash    string
	Status      int
	Body        []byte
	ContentType string
	ExpiresAt   time.Time
}

type responseCapture struct {
	h      http.Header
	body   bytes.Buffer
	status int
}

func (r *responseCapture) Header() http.Header {
	return r.h
}

func (r *responseCapture) WriteHeader(statusCode int) {
	if r.status == 0 {
		r.status = statusCode
	}
}

func (r *responseCapture) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func NewIdempotencyStore(maxEntries int) *IdempotencyStore {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &IdempotencyStore{items: make(map[string]idemRecord, maxEntries), maxSize: maxEntries}
}

func IdempotencyKey(cfg IdempotencyConfig) Middleware {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "Idempotency-Key"
	}
	replayHeader := cfg.ReplayHeader
	if replayHeader == "" {
		replayHeader = "Idempotency-Replayed"
	}
	methods := cfg.TrackedMethods
	if len(methods) == 0 {
		methods = map[string]struct{}{"POST": {}, "PUT": {}, "PATCH": {}}
	}
	store := cfg.Store
	if store == nil {
		store = NewIdempotencyStore(cfg.MaxEntries)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := methods[r.Method]; !ok {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(headerName)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
			if err != nil || int64(len(body)) > maxBody {
				WriteError(w, ErrPayloadTooLarge, nil)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := sha256.Sum256(body)
			bodyHash := hex.EncodeToString(hash[:])
			storeKey := r.Method + "|" + r.URL.Path + "|" + key

			if rec, ok := store.get(storeKey); ok {
				if rec.BodyHash != bodyHash {
					WriteError(w, newErr(ErrConflict.Status, "idempotency_conflict", "Same idempotency key used with different payload"), nil)
					return
				}
				if rec.ContentType != "" {
					w.Header().Set("Content-Type", rec.ContentType)
				}
				w.Header().Set(replayHeader, "true")
				w.WriteHeader(rec.Status)
				_, _ = w.Write(rec.Body)
				return
			}

			cap := &responseCapture{h: make(http.Header)}
			next.ServeHTTP(cap, r)

			for hk, vals := range cap.h {
				for _, v := range vals {
					w.Header().Add(hk, v)
				}
			}
			w.Header().Set(replayHeader, "false")
			status := cap.status
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			_, _ = w.Write(cap.body.Bytes())

			store.put(storeKey, idemRecord{
				BodyHash:    bodyHash,
				Status:      status,
				Body:        append([]byte(nil), cap.body.Bytes()...),
				ContentType: cap.h.Get("Content-Type"),
				ExpiresAt:   time.Now().Add(ttl),
			})
		})
	}
}

func (s *IdempotencyStore) get(key string) (idemRecord, bool) {
	now := time.Now()
	s.mu.RLock()
	rec, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return idemRecord{}, false
	}
	if now.After(rec.ExpiresAt) {
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		return idemRecord{}, false
	}
	return rec, true
}

func (s *IdempotencyStore) put(key string, rec idemRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.maxSize {
		// Deterministic lightweight eviction: remove one expired item first,
		// otherwise remove first encountered item.
		now := time.Now()
		for k, v := range s.items {
			if now.After(v.ExpiresAt) {
				delete(s.items, k)
				break
			}
			delete(s.items, k)
			break
		}
	}
	s.items[key] = rec
}
