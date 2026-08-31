package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/api"
	"github.com/priyanshuguptadev/job-board/internal/auth"
	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/logger"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/storage"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/priyanshuguptadev/job-board/internal/webhook"
)

func main() {
	if len(os.Args) < 2 {
		runServer()
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "server":
		runServer()
	case "migrate":
		runMigrate(os.Args[2:])
	case "keygen":
		runKeygen(os.Args[2:])
	case "seed":
		runSeed(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Job Board API - Headless Job Board Service")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  jobboard server                         Start HTTP server and run auto-migrations")
	fmt.Println("  jobboard migrate up                     Run all pending migrations")
	fmt.Println("  jobboard migrate down [steps]           Roll back migrations (default 1 step)")
	fmt.Println("  jobboard migrate status                 Show current migration version")
	fmt.Println("  jobboard keygen --name <n> --scope <s>  Generate an API key (scope: admin|public)")
	fmt.Println("  jobboard seed                           Bootstrap initial admin and public API keys")
	fmt.Println()
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	l := logger.New(cfg.Server.Env, cfg.Server.LogLevel)
	slog.SetDefault(l)
	l.Info("Starting Job Board API",
		"env", cfg.Server.Env,
		"port", cfg.Server.Port,
		"log_level", cfg.Server.LogLevel,
	)

	var db *sql.DB
	if cfg.Database.URL != "" {
		l.Info("Connecting to PostgreSQL database...")
		var err error
		db, err = postgres.NewDB(cfg.Database)
		if err != nil {
			l.Error("Failed to connect to database", "error", err)
			os.Exit(1)
		}
		defer func() {
			l.Info("Closing database connections...")
			_ = db.Close()
		}()

		l.Info("Running database auto-migrations...")
		if err := postgres.MigrateUp(db); err != nil {
			l.Error("Failed to run database migrations", "error", err)
			os.Exit(1)
		}
		l.Info("Database migrations applied successfully")
	} else {
		l.Warn("DATABASE_URL is not set; running without database connection")
	}

	var strg storage.Storage
	if cfg.S3.Bucket != "" {
		var err error
		strg, err = storage.NewS3Storage(context.Background(), cfg.S3)
		if err != nil {
			l.Error("Failed to initialize S3 storage", "error", err)
			os.Exit(1)
		}
		l.Info("S3 storage initialized", "bucket", cfg.S3.Bucket)
	} else {
		l.Warn("S3_BUCKET is not set; using in-memory storage")
		strg = storage.NewMemoryStorage()
	}

	// Server run context for graceful shutdown
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	var apiKeyRepo domain.ApiKeyRepository
	var jobRepo domain.JobRepository
	var appRepo domain.ApplicationRepository
	var appNoteRepo domain.ApplicationNoteRepository
	var webhookRepo domain.WebhookSubscriptionRepository
	var webhookDispatcher webhook.Dispatcher
	var webhookService service.WebhookService
	var jobService service.JobService
	var appService service.ApplicationService

	if db != nil {
		apiKeyRepo = postgres.NewApiKeyRepository(db)
		jobRepo = postgres.NewJobRepository(db)
		appRepo = postgres.NewApplicationRepository(db)
		appNoteRepo = postgres.NewApplicationNoteRepository(db)
		webhookRepo = postgres.NewWebhookSubscriptionRepository(db)

		webhookDispatcher = webhook.NewDispatcher(webhookRepo, &webhook.DispatcherConfig{
			Logger: l,
		})
		webhookDispatcher.Start(serverCtx)
		defer webhookDispatcher.Stop()

		webhookService = service.NewWebhookService(webhookRepo, webhookDispatcher)
		jobService = service.NewJobService(jobRepo, webhookService)
		appService = service.NewApplicationService(jobRepo, appRepo, appNoteRepo, strg, webhookService)
	}

	router := api.NewRouter(api.RouterConfig{
		Config:         cfg,
		Logger:         l,
		DB:             db,
		ApiKeyRepo:     apiKeyRepo,
		JobService:     jobService,
		AppService:     appService,
		WebhookService: webhookService,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig
		l.Info("Shutdown signal received, initiating graceful shutdown...")

		// Shutdown signal with grace period of 10 seconds
		shutdownCtx, cancel := context.WithTimeout(serverCtx, 10*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				l.Error("Graceful shutdown timed out, forcing exit")
				os.Exit(1)
			}
		}()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			l.Error("Server shutdown error", "error", err)
		}
		serverStopCtx()
	}()

	l.Info("HTTP server listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		l.Error("Server listen failed", "error", err)
		os.Exit(1)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
	l.Info("Server stopped successfully")
}

func runMigrate(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: jobboard migrate [up|down <steps>|status]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if cfg.Database.URL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required to run migrations")
		os.Exit(1)
	}

	db, err := postgres.NewDB(cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	action := args[0]
	switch action {
	case "up":
		fmt.Println("Applying pending database migrations...")
		if err := postgres.MigrateUp(db); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully.")

	case "down":
		steps := 1
		if len(args) > 1 {
			if s, err := strconv.Atoi(args[1]); err == nil && s > 0 {
				steps = s
			}
		}
		fmt.Printf("Rolling back %d database migration(s)...\n", steps)
		if err := postgres.MigrateDown(db, steps); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Rollback completed successfully.")

	case "status":
		version, dirty, err := postgres.MigrationVersion(db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get migration status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Current migration version: %d (dirty: %t)\n", version, dirty)

	default:
		fmt.Fprintf(os.Stderr, "Unknown migrate command: %s\n", action)
		os.Exit(1)
	}
}

func runKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	name := fs.String("name", "", "Name or description for the API key")
	scope := fs.String("scope", "", "Scope for the API key: admin or public")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse keygen flags: %v\n", err)
		os.Exit(1)
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "Error: --name is required")
		os.Exit(1)
	}

	if *scope != "admin" && *scope != "public" {
		fmt.Fprintln(os.Stderr, "Error: --scope must be either 'admin' or 'public'")
		os.Exit(1)
	}

	token, apiKey, err := auth.GenerateKey(*name, domain.ApiKeyScope(*scope))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate API key: %v\n", err)
		os.Exit(1)
	}

	cfg, _ := config.Load()
	if cfg != nil && cfg.Database.URL != "" {
		db, err := postgres.NewDB(cfg.Database)
		if err == nil {
			defer db.Close()
			keyRepo := postgres.NewApiKeyRepository(db)
			err = keyRepo.Create(context.Background(), apiKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist API key to database: %v\n", err)
			} else {
				fmt.Println("API key successfully saved to database.")
			}
		}
	}

	fmt.Println()
	fmt.Println("=================================================================")
	fmt.Printf("Key Name : %s\n", apiKey.Name)
	fmt.Printf("Scope    : %s\n", apiKey.Scope)
	fmt.Printf("Prefix   : %s\n", apiKey.KeyPrefix)
	fmt.Printf("Key Hash : %s\n", apiKey.KeyHash)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("API Key  : %s\n", token)
	fmt.Println("=================================================================")
	fmt.Println("Store this key safely! It will NOT be displayed again.")
	fmt.Println()
}

func runSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	adminName := fs.String("admin-name", "Initial Admin Key", "Name for the admin API key")
	pubName := fs.String("public-name", "Default Public Key", "Name for the public API key")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse seed flags: %v\n", err)
		os.Exit(1)
	}

	adminToken, adminApiKey, err := auth.GenerateKey(*adminName, domain.ApiKeyScopeAdmin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate admin API key: %v\n", err)
		os.Exit(1)
	}

	pubToken, pubApiKey, err := auth.GenerateKey(*pubName, domain.ApiKeyScopePublic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate public API key: %v\n", err)
		os.Exit(1)
	}

	cfg, _ := config.Load()
	if cfg != nil && cfg.Database.URL != "" {
		db, err := postgres.NewDB(cfg.Database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not connect to database (%v); generated keys were not persisted.\n", err)
		} else {
			defer db.Close()

			// Run auto migrations if needed
			if err := postgres.MigrateUp(db); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto-migration failed: %v\n", err)
			}

			keyRepo := postgres.NewApiKeyRepository(db)
			ctx := context.Background()

			savedAdmin := false
			savedPub := false

			if err := keyRepo.Create(ctx, adminApiKey); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist admin API key to database: %v\n", err)
			} else {
				savedAdmin = true
			}
			if err := keyRepo.Create(ctx, pubApiKey); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist public API key to database: %v\n", err)
			} else {
				savedPub = true
			}

			if savedAdmin && savedPub {
				fmt.Println("Initial API keys successfully saved to database.")
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: DATABASE_URL not set; generated keys are not persisted to database.")
	}

	fmt.Println()
	fmt.Println("=================================================================")
	fmt.Println("                   BOOTSTRAPPED API KEYS                        ")
	fmt.Println("=================================================================")
	fmt.Printf("1. ADMIN API KEY (%s)\n", adminApiKey.Name)
	fmt.Printf("   Scope  : %s\n", adminApiKey.Scope)
	fmt.Printf("   Prefix : %s\n", adminApiKey.KeyPrefix)
	fmt.Printf("   Token  : %s\n", adminToken)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("2. PUBLIC API KEY (%s)\n", pubApiKey.Name)
	fmt.Printf("   Scope  : %s\n", pubApiKey.Scope)
	fmt.Printf("   Prefix : %s\n", pubApiKey.KeyPrefix)
	fmt.Printf("   Token  : %s\n", pubToken)
	fmt.Println("=================================================================")
	fmt.Println("Store these keys safely! Tokens will NOT be displayed again.")
	fmt.Println()
}
