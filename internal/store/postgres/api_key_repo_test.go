package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApiKeyRepository(t *testing.T) {
	db := getTestDB(t)
	repo := postgres.NewApiKeyRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByHash", func(t *testing.T) {
		key := &domain.ApiKey{
			Name:      "Admin Test Key",
			KeyHash:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			KeyPrefix: "jb_sec_",
			Scope:     domain.ApiKeyScopeAdmin,
		}

		err := repo.Create(ctx, key)
		require.NoError(t, err)
		assert.NotEmpty(t, key.ID)
		assert.False(t, key.CreatedAt.IsZero())

		fetched, err := repo.GetByHash(ctx, key.KeyHash)
		require.NoError(t, err)
		assert.Equal(t, key.ID, fetched.ID)
		assert.Equal(t, key.Name, fetched.Name)
		assert.Equal(t, key.KeyPrefix, fetched.KeyPrefix)
		assert.Equal(t, domain.ApiKeyScopeAdmin, fetched.Scope)
		assert.Nil(t, fetched.LastUsedAt)
	})

	t.Run("Create duplicate hash returns ErrConflict", func(t *testing.T) {
		key1 := &domain.ApiKey{
			Name:      "Key 1",
			KeyHash:   "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			KeyPrefix: "jb_pub_",
			Scope:     domain.ApiKeyScopePublic,
		}
		require.NoError(t, repo.Create(ctx, key1))

		key2 := &domain.ApiKey{
			Name:      "Key 2",
			KeyHash:   "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			KeyPrefix: "jb_pub_",
			Scope:     domain.ApiKeyScopePublic,
		}
		err := repo.Create(ctx, key2)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("GetByID and GetByID not found", func(t *testing.T) {
		key := &domain.ApiKey{
			Name:      "Public Key",
			KeyHash:   "1111111111111111111111111111111111111111111111111111111111111111",
			KeyPrefix: "jb_pub_",
			Scope:     domain.ApiKeyScopePublic,
		}
		require.NoError(t, repo.Create(ctx, key))

		fetched, err := repo.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, key.ID, fetched.ID)

		_, err = repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("UpdateLastUsed", func(t *testing.T) {
		key := &domain.ApiKey{
			Name:      "Key to update",
			KeyHash:   "2222222222222222222222222222222222222222222222222222222222222222",
			KeyPrefix: "jb_sec_",
			Scope:     domain.ApiKeyScopeAdmin,
		}
		require.NoError(t, repo.Create(ctx, key))

		now := time.Now().Truncate(time.Microsecond)
		err := repo.UpdateLastUsed(ctx, key.ID, now)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, key.ID)
		require.NoError(t, err)
		require.NotNil(t, fetched.LastUsedAt)
		assert.WithinDuration(t, now, *fetched.LastUsedAt, time.Second)

		err = repo.UpdateLastUsed(ctx, "00000000-0000-0000-0000-000000000000", now)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("List and Delete", func(t *testing.T) {
		cleanTables(t, db)

		k1 := &domain.ApiKey{Name: "K1", KeyHash: "3333333333333333333333333333333333333333333333333333333333333333", KeyPrefix: "jb_pub_", Scope: domain.ApiKeyScopePublic}
		k2 := &domain.ApiKey{Name: "K2", KeyHash: "4444444444444444444444444444444444444444444444444444444444444444", KeyPrefix: "jb_sec_", Scope: domain.ApiKeyScopeAdmin}
		require.NoError(t, repo.Create(ctx, k1))
		require.NoError(t, repo.Create(ctx, k2))

		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 2)

		err = repo.Delete(ctx, k1.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, k1.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = repo.Delete(ctx, k1.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
