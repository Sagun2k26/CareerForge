package server

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

// rateLimiter is a minimal per-IP fixed-window limiter. It avoids an external
// dependency while still demonstrating request throttling in the middleware
// chain.
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string]int
	window  time.Duration
	limit   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		hits:    make(map[string]int),
		window:  window,
		limit:   limit,
		resetAt: time.Now().Add(window),
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if time.Now().After(rl.resetAt) {
		rl.hits = make(map[string]int)
		rl.resetAt = time.Now().Add(rl.window)
	}
	rl.hits[ip]++
	return rl.hits[ip] <= rl.limit
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.allow(ip) {
			httpx.Error(w, httpx.NewError(http.StatusTooManyRequests, "rate_limited", "too many requests, slow down"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
