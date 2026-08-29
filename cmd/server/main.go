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
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
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

	router := api.NewRouter(api.RouterConfig{
		Config: cfg,
		Logger: l,
		DB:     db,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context for graceful shutdown
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

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
