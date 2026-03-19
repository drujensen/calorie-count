package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// bucket holds the token bucket state for a single IP address.
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// RateLimiter is a per-IP token bucket rate limiter.
type RateLimiter struct {
	rate    float64 // tokens per second (requestsPerMinute / 60)
	burst   float64 // max tokens (== requestsPerMinute)
	mu      sync.Mutex
	buckets map[string]*bucket
}

// NewRateLimiter creates a RateLimiter allowing requestsPerMinute requests per
// IP per minute. ctx is used to stop the background cleanup goroutine.
func NewRateLimiter(ctx context.Context, requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		rate:    float64(requestsPerMinute) / 60.0,
		burst:   float64(requestsPerMinute),
		buckets: make(map[string]*bucket),
	}

	go rl.cleanupLoop(ctx)
	return rl
}

// Limit returns middleware that enforces the rate limit. Requests that exceed
// the limit receive a 429 Too Many Requests response.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r.RemoteAddr)
		b := rl.getBucket(ip)

		b.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(b.lastRefill).Seconds()
		b.tokens += elapsed * rl.rate
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastRefill = now
		b.lastSeen = now

		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		b.mu.Unlock()

		if !allowed {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getBucket returns the bucket for the given IP, creating one if needed.
func (rl *RateLimiter) getBucket(ip string) *bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{
			tokens:     rl.burst,
			lastRefill: time.Now(),
			lastSeen:   time.Now(),
		}
		rl.buckets[ip] = b
	}
	return b
}

// cleanupLoop removes buckets not seen in the last 5 minutes. Runs until ctx
// is cancelled.
func (rl *RateLimiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-5 * time.Minute)
			rl.mu.Lock()
			for ip, b := range rl.buckets {
				b.mu.Lock()
				idle := b.lastSeen.Before(cutoff)
				b.mu.Unlock()
				if idle {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// extractIP strips the port from a host:port RemoteAddr string.
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
