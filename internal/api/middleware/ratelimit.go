package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages per-IP token bucket rate limiters in-memory.
type IPRateLimiter struct {
	mu              sync.RWMutex
	visitors        map[string]*visitor
	rps             rate.Limit
	burst           int
	visitorTTL      time.Duration
	cleanupInterval time.Duration
	stopChan        chan struct{}
	stopOnce        sync.Once
}

// NewIPRateLimiter creates a new IPRateLimiter with background visitor cleanup.
func NewIPRateLimiter(rps rate.Limit, burst int) *IPRateLimiter {
	return NewIPRateLimiterWithTTL(rps, burst, 3*time.Minute, 1*time.Minute)
}

// NewIPRateLimiterWithTTL creates an IPRateLimiter with custom TTL and cleanup interval.
func NewIPRateLimiterWithTTL(rps rate.Limit, burst int, ttl, cleanupInterval time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		visitors:        make(map[string]*visitor),
		rps:             rps,
		burst:           burst,
		visitorTTL:      ttl,
		cleanupInterval: cleanupInterval,
		stopChan:        make(chan struct{}),
	}

	go limiter.startCleanupLoop()

	return limiter
}

func (l *IPRateLimiter) startCleanupLoop() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.Cleanup()
		case <-l.stopChan:
			return
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (l *IPRateLimiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopChan)
	})
}

// Cleanup removes stale IP entries that have not made requests within visitorTTL.
func (l *IPRateLimiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > l.visitorTTL {
			delete(l.visitors, ip)
		}
	}
}

// GetLimiter returns or creates a rate.Limiter for the given IP address.
func (l *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, exists := l.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(l.rps, l.burst)
		l.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// Allow checks if the given IP address is allowed to process a request.
func (l *IPRateLimiter) Allow(ip string) bool {
	return l.GetLimiter(ip).Allow()
}

// Handler returns a standard net/http middleware handler.
func (l *IPRateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		if !l.Allow(ip) {
			w.Header().Set("Retry-After", "1")
			respondJSONError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit burst or requests per second exceeded. Please try again later.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimit creates an HTTP middleware that limits requests by IP address.
func RateLimit(rps int, burst int) func(next http.Handler) http.Handler {
	limiter := NewIPRateLimiter(rate.Limit(rps), burst)
	return limiter.Handler
}

// getIP extracts the client's IP address from the request.
func getIP(r *http.Request) string {
	// RemoteAddr is guaranteed to be populated; RealIP middleware might also have normalized it
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}

	// Fallback to headers if RemoteAddr is somehow empty
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	return "127.0.0.1"
}
