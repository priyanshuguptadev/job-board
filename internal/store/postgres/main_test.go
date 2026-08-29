package postgres_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
)

var testDB *sql.DB

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: TEST_DATABASE_URL / DATABASE_URL not set")
	}

	if testDB != nil {
		cleanTables(t, testDB)
		return testDB
	}

	cfg := config.DatabaseConfig{
		URL:          dbURL,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	db, err := postgres.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := postgres.MigrateUp(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	testDB = db
	cleanTables(t, testDB)
	return testDB
}

func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()
	queries := []string{
		"TRUNCATE TABLE webhook_subscriptions CASCADE",
		"TRUNCATE TABLE application_notes CASCADE",
		"TRUNCATE TABLE applications CASCADE",
		"TRUNCATE TABLE jobs CASCADE",
		"TRUNCATE TABLE api_keys CASCADE",
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("Failed to truncate table (%s): %v", q, err)
		}
	}
}
