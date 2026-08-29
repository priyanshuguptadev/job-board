package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a configured CORS middleware handler using go-chi/cors.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
			"X-JobBoard-Signature",
			"Origin",
		},
		ExposedHeaders: []string{
			"Link",
			"Location",
			"Retry-After",
			"X-Total-Count",
		},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
