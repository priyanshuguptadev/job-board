package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer recovers from panics, logs stack traces, and returns a standard JSON error response.
func Recoverer(l *slog.Logger) func(next http.Handler) http.Handler {
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

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "INTERNAL_SERVER_ERROR",
							"message": "An unexpected server error occurred.",
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
