package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests within burst limit", func(t *testing.T) {
		limiter := NewIPRateLimiter(rate.Limit(5), 3)
		defer limiter.Stop()

		handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
			req.RemoteAddr = "192.168.1.100:12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
		}
	})

	t.Run("blocks requests exceeding burst limit with 429", func(t *testing.T) {
		limiter := NewIPRateLimiter(rate.Limit(1), 2)
		defer limiter.Stop()

		handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// First 2 requests should pass
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
			req.RemoteAddr = "192.168.1.101:12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		// 3rd immediate request must be rate limited
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.RemoteAddr = "192.168.1.101:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "1", rec.Header().Get("Retry-After"))

		var resp map[string]map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "RATE_LIMIT_EXCEEDED", resp["error"]["code"])
		assert.Contains(t, resp["error"]["message"], "Rate limit")
	})

	t.Run("different IPs have independent buckets", func(t *testing.T) {
		limiter := NewIPRateLimiter(rate.Limit(1), 1)
		defer limiter.Stop()

		handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// IP A: request 1 (pass)
		reqA1 := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		reqA1.RemoteAddr = "10.0.0.1:1234"
		recA1 := httptest.NewRecorder()
		handler.ServeHTTP(recA1, reqA1)
		assert.Equal(t, http.StatusOK, recA1.Code)

		// IP A: request 2 (blocked)
		reqA2 := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		reqA2.RemoteAddr = "10.0.0.1:1234"
		recA2 := httptest.NewRecorder()
		handler.ServeHTTP(recA2, reqA2)
		assert.Equal(t, http.StatusTooManyRequests, recA2.Code)

		// IP B: request 1 (pass, not affected by IP A)
		reqB1 := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		reqB1.RemoteAddr = "10.0.0.2:1234"
		recB1 := httptest.NewRecorder()
		handler.ServeHTTP(recB1, reqB1)
		assert.Equal(t, http.StatusOK, recB1.Code)
	})

	t.Run("extracts IP from RemoteAddr and fallback headers", func(t *testing.T) {
		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		req1.RemoteAddr = "203.0.113.195:8080"
		assert.Equal(t, "203.0.113.195", getIP(req1))

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = ""
		req2.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.1")
		assert.Equal(t, "198.51.100.1", getIP(req2))

		req3 := httptest.NewRequest(http.MethodGet, "/", nil)
		req3.RemoteAddr = ""
		req3.Header.Set("X-Real-IP", "198.51.100.2")
		assert.Equal(t, "198.51.100.2", getIP(req3))

		req4 := httptest.NewRequest(http.MethodGet, "/", nil)
		req4.RemoteAddr = ""
		assert.Equal(t, "127.0.0.1", getIP(req4))
	})

	t.Run("cleanup removes expired visitors", func(t *testing.T) {
		limiter := NewIPRateLimiterWithTTL(rate.Limit(10), 10, 50*time.Millisecond, 200*time.Millisecond)
		defer limiter.Stop()

		limiter.GetLimiter("1.2.3.4")
		limiter.GetLimiter("5.6.7.8")

		limiter.mu.RLock()
		assert.Len(t, limiter.visitors, 2)
		limiter.mu.RUnlock()

		time.Sleep(100 * time.Millisecond)
		limiter.Cleanup()

		limiter.mu.RLock()
		assert.Len(t, limiter.visitors, 0)
		limiter.mu.RUnlock()
	})

	t.Run("RateLimit helper function works", func(t *testing.T) {
		mw := RateLimit(10, 5)
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
