package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger returns a middleware that logs incoming HTTP requests using structured slog.
func Logger(l *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				status := ww.Status()
				bytesWritten := ww.BytesWritten()

				logLevel := slog.LevelInfo
				if status >= 500 {
					logLevel = slog.LevelError
				} else if status >= 400 {
					logLevel = slog.LevelWarn
				}

				l.Log(r.Context(), logLevel, "HTTP request handled",
					"method", r.Method,
					"path", r.URL.Path,
					"status", status,
					"duration_ms", duration.Milliseconds(),
					"bytes", bytesWritten,
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
