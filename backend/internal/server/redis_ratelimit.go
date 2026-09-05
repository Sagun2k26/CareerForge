package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

func distributedRateLimiter(
	redisURL string,
	limit int,
	window time.Duration,
	logger *slog.Logger,
) (func(http.Handler) http.Handler, func()) {
	if redisURL == "" {
		return newRateLimiter(limit, window).middleware, func() {}
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Warn("invalid REDIS_URL; using in-memory rate limiter", "error", err)
		return newRateLimiter(limit, window).middleware, func() {}
	}
	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("redis unavailable; using in-memory rate limiter", "error", err)
		_ = client.Close()
		return newRateLimiter(limit, window).middleware, func() {}
	}

	script := redis.NewScript(`
		local current = redis.call("INCR", KEYS[1])
		if current == 1 then
			redis.call("PEXPIRE", KEYS[1], ARGV[1])
		end
		return current
	`)

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			key := "careerforge:ratelimit:" + host

			ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
			defer cancel()

			n, err := script.Run(ctx, client, []string{key}, window.Milliseconds()).Int()
			if err != nil {
				logger.Warn("redis rate-limit check failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			if n > limit {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	logger.Info("distributed rate limiter enabled", "backend", "redis")
	return mw, func() { _ = client.Close() }
}
