package postgres_test

import (
	"context"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookSubscriptionRepository(t *testing.T) {
	db := getTestDB(t)
	repo := postgres.NewWebhookSubscriptionRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		cleanTables(t, db)

		sub := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/webhooks/jobs",
			SecretToken: "supersecrettoken123",
			Events:      []string{domain.EventJobPublished, domain.EventJobArchived},
			IsActive:    true,
		}

		err := repo.Create(ctx, sub)
		require.NoError(t, err)
		assert.NotEmpty(t, sub.ID)
		assert.False(t, sub.CreatedAt.IsZero())
		assert.False(t, sub.UpdatedAt.IsZero())

		fetched, err := repo.GetByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, sub.ID, fetched.ID)
		assert.Equal(t, "https://example.com/webhooks/jobs", fetched.TargetURL)
		assert.Equal(t, "supersecrettoken123", fetched.SecretToken)
		assert.ElementsMatch(t, []string{domain.EventJobPublished, domain.EventJobArchived}, fetched.Events)
		assert.True(t, fetched.IsActive)
	})

	t.Run("GetByID not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ListActiveByEvent", func(t *testing.T) {
		cleanTables(t, db)

		sub1 := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/webhook1",
			SecretToken: "s1",
			Events:      []string{domain.EventJobPublished},
			IsActive:    true,
		}
		sub2 := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/webhook2",
			SecretToken: "s2",
			Events:      []string{domain.EventApplicationCreated, domain.EventApplicationStageUpdated},
			IsActive:    true,
		}
		sub3 := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/webhook3",
			SecretToken: "s3",
			Events:      []string{"*"}, // Wildcard
			IsActive:    true,
		}
		sub4 := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/webhook4",
			SecretToken: "s4",
			Events:      []string{domain.EventJobPublished},
			IsActive:    false, // Inactive
		}

		require.NoError(t, repo.Create(ctx, sub1))
		require.NoError(t, repo.Create(ctx, sub2))
		require.NoError(t, repo.Create(ctx, sub3))
		require.NoError(t, repo.Create(ctx, sub4))

		// Query job.published -> should match sub1 and sub3 (wildcard), but not sub4 (inactive) or sub2
		subs, err := repo.ListActiveByEvent(ctx, domain.EventJobPublished)
		require.NoError(t, err)
		assert.Len(t, subs, 2)
		ids := []string{subs[0].ID, subs[1].ID}
		assert.Contains(t, ids, sub1.ID)
		assert.Contains(t, ids, sub3.ID)

		// Query application.created -> should match sub2 and sub3
		subs, err = repo.ListActiveByEvent(ctx, domain.EventApplicationCreated)
		require.NoError(t, err)
		assert.Len(t, subs, 2)
		ids = []string{subs[0].ID, subs[1].ID}
		assert.Contains(t, ids, sub2.ID)
		assert.Contains(t, ids, sub3.ID)
	})

	t.Run("Update and Delete", func(t *testing.T) {
		cleanTables(t, db)

		sub := &domain.WebhookSubscription{
			TargetURL:   "https://example.com/initial",
			SecretToken: "token1",
			Events:      []string{domain.EventJobPublished},
			IsActive:    true,
		}
		require.NoError(t, repo.Create(ctx, sub))

		// Update
		sub.TargetURL = "https://example.com/updated"
		sub.Events = []string{domain.EventJobPublished, domain.EventApplicationCreated}
		sub.IsActive = false

		err := repo.Update(ctx, sub)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/updated", fetched.TargetURL)
		assert.ElementsMatch(t, []string{domain.EventJobPublished, domain.EventApplicationCreated}, fetched.Events)
		assert.False(t, fetched.IsActive)

		// Update not found
		sub.ID = "00000000-0000-0000-0000-000000000000"
		err = repo.Update(ctx, sub)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Delete
		err = repo.Delete(ctx, fetched.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, fetched.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = repo.Delete(ctx, fetched.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
