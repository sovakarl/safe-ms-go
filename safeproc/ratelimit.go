package safeproc

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimitConfig struct {
	RatePerSecond float64
	Burst         float64
	EntryTTL      time.Duration
	KeyFunc       func(*http.Request) string
}

type rateEntry struct {
	tokens   float64
	lastSeen time.Time
}

type rateLimiter struct {
	mu    sync.Mutex
	m     map[string]rateEntry
	cfg   RateLimitConfig
	nowFn func() time.Time
}

func RateLimit(cfg RateLimitConfig) Middleware {
	if cfg.RatePerSecond <= 0 {
		cfg.RatePerSecond = 20
	}
	if cfg.Burst <= 0 {
		cfg.Burst = math.Max(5, cfg.RatePerSecond)
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = 10 * time.Minute
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = clientIPKey
	}

	rl := &rateLimiter{
		m:     make(map[string]rateEntry, 1000),
		cfg:   cfg,
		nowFn: time.Now,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)
			if key == "" {
				key = "unknown"
			}
			if !rl.allow(key) {
				w.Header().Set("Retry-After", "1")
				WriteError(w, ErrTooManyRequests, nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (r *rateLimiter) allow(key string) bool {
	now := r.nowFn()
	r.mu.Lock()
	defer r.mu.Unlock()

	r.gc(now)

	entry, ok := r.m[key]
	if !ok {
		r.m[key] = rateEntry{tokens: r.cfg.Burst - 1, lastSeen: now}
		return true
	}

	elapsed := now.Sub(entry.lastSeen).Seconds()
	entry.tokens = math.Min(r.cfg.Burst, entry.tokens+elapsed*r.cfg.RatePerSecond)
	entry.lastSeen = now
	if entry.tokens < 1 {
		r.m[key] = entry
		return false
	}
	entry.tokens -= 1
	r.m[key] = entry
	return true
}

func (r *rateLimiter) gc(now time.Time) {
	for k, v := range r.m {
		if now.Sub(v.lastSeen) > r.cfg.EntryTTL {
			delete(r.m, k)
		}
	}
}

func clientIPKey(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}
	return req.RemoteAddr
}
