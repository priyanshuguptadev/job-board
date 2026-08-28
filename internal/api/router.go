package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/priyanshuguptadev/job-board/internal/api/middleware"
	"github.com/priyanshuguptadev/job-board/internal/config"
)

// RouterConfig contains dependencies and configuration for setting up the router.
type RouterConfig struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *sql.DB
}

// NewRouter initializes and returns a Chi router configured with middlewares and routes.
func NewRouter(rc RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Base middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(rc.Logger))
	r.Use(middleware.Recoverer(rc.Logger))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   rc.Config.Server.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-JobBoard-Signature"},
		ExposedHeaders:   []string{"Link", "Location"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Standard error handlers for 404 & 405
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		RespondError(w, http.StatusNotFound, ErrCodeNotFound, "The requested resource was not found.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		RespondError(w, http.StatusMethodNotAllowed, ErrCodeNotFound, "Method not allowed on this endpoint.")
	})

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "disabled"

		if rc.DB != nil {
			if err := rc.DB.PingContext(r.Context()); err != nil {
				RespondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
					"status":   "unhealthy",
					"database": "unreachable",
					"error":    err.Error(),
				})
				return
			}
			dbStatus = "connected"
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":   status,
			"database": dbStatus,
		})
	})

	return r
}
