package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/priyanshuguptadev/job-board/migrations"
)

// newMigrator creates a new *migrate.Migrate instance with the embedded migrations.
func newMigrator(db *sql.DB) (*migrate.Migrate, error) {
	d, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	driver, err := pgx.WithInstance(db, &pgx.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver for migrations: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "pgx5", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return m, nil
}

// MigrateUp applies all pending database migrations.
func MigrateUp(db *sql.DB) error {
	m, err := newMigrator(db)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up failed: %w", err)
	}

	return nil
}

// MigrateDown rolls back database migrations by the specified number of steps (default 1).
func MigrateDown(db *sql.DB, steps int) error {
	if steps <= 0 {
		steps = 1
	}

	m, err := newMigrator(db)
	if err != nil {
		return err
	}

	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration down failed: %w", err)
	}

	return nil
}

// MigrationVersion returns the current applied migration version and dirty flag.
func MigrationVersion(db *sql.DB) (uint, bool, error) {
	m, err := newMigrator(db)
	if err != nil {
		return 0, false, err
	}

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}
