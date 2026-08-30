package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/priyanshuguptadev/job-board/internal/api/middleware"
	v1 "github.com/priyanshuguptadev/job-board/internal/api/v1"
	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
)

// RouterConfig contains dependencies and configuration for setting up the router.
type RouterConfig struct {
	Config     *config.Config
	Logger     *slog.Logger
	DB         *sql.DB
	ApiKeyRepo domain.ApiKeyRepository
	JobService service.JobService
	AppService service.ApplicationService
}

// NewRouter initializes and returns a Chi router configured with middlewares and routes.
func NewRouter(rc RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	apiKeyRepo := rc.ApiKeyRepo
	jobService := rc.JobService
	appService := rc.AppService

	// Base middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(rc.Logger))
	r.Use(middleware.Recoverer(rc.Logger))
	r.Use(middleware.CORS(rc.Config.Server.CORSAllowedOrigins))

	// Standard error handlers for 404 & 405
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		RespondError(w, http.StatusNotFound, ErrCodeNotFound, "The requested resource was not found.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		RespondError(w, http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed, "Method not allowed on this endpoint.")
	})

	// System & Observability endpoints
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

	// API v1 Subrouter
	r.Route("/v1", func(v1Router chi.Router) {
		// Public routes (Rate limited + Public API Key scope)
		v1Router.Route("/public", func(public chi.Router) {
			public.Use(middleware.RateLimit(rc.Config.Server.RateLimitRPS, rc.Config.Server.RateLimitBurst))
			if apiKeyRepo != nil {
				public.Use(middleware.ApiKeyAuth(apiKeyRepo, domain.ApiKeyScopePublic, rc.Logger))
			}

			public.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
				RespondJSON(w, http.StatusOK, map[string]string{"message": "pong"})
			})

			if jobService != nil && appService != nil {
				pubHandler := v1.NewPublicHandler(jobService, appService, rc.Logger)
				public.Get("/jobs", pubHandler.ListJobs)
				public.Get("/jobs/{slug_or_id}", pubHandler.GetJob)
				public.Get("/departments", pubHandler.ListDepartments)
				public.Post("/jobs/{job_id}/apply", pubHandler.Apply)
			}
		})

		// Admin routes (Admin API Key scope)
		v1Router.Route("/admin", func(admin chi.Router) {
			if apiKeyRepo != nil {
				admin.Use(middleware.ApiKeyAuth(apiKeyRepo, domain.ApiKeyScopeAdmin, rc.Logger))
			}

			admin.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
				RespondJSON(w, http.StatusOK, map[string]string{"message": "pong"})
			})
		})
	})

	return r
}
