package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer recovers from panics, logs stack traces, and returns a standard JSON error response.
func Recoverer(l *slog.Logger) func(next http.Handler) http.Handler {
	if l == nil {
		l = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						panic(rvr)
					}

					stack := string(debug.Stack())
					l.Error("panic recovered",
						"error", fmt.Sprintf("%v", rvr),
						"stack", stack,
						"path", r.URL.Path,
						"method", r.Method,
					)

					respondJSONError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "An unexpected server error occurred.")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
