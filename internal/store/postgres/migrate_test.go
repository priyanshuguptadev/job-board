package postgres_test

import (
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	db := getTestDB(t)

	// After getTestDB, MigrateUp has run. Check version.
	version, dirty, err := postgres.MigrationVersion(db)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, uint(1))
	assert.False(t, dirty)

	// Running MigrateUp again should be a no-op / no error.
	err = postgres.MigrateUp(db)
	require.NoError(t, err)
}
