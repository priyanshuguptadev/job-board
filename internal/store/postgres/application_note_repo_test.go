package postgres_test

import (
	"context"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationNoteRepository(t *testing.T) {
	db := getTestDB(t)
	jobRepo := postgres.NewJobRepository(db)
	appRepo := postgres.NewApplicationRepository(db)
	noteRepo := postgres.NewApplicationNoteRepository(db)
	ctx := context.Background()

	cleanTables(t, db)

	job := &domain.Job{
		Slug:                "staff-engineer",
		Title:               "Staff Engineer",
		Department:          "Engineering",
		Location:            "Remote",
		DescriptionMarkdown: "desc",
		Status:              domain.JobStatusPublished,
	}
	require.NoError(t, jobRepo.Create(ctx, job))

	app := &domain.Application{
		JobID:          job.ID,
		CandidateName:  "Test Candidate",
		CandidateEmail: "candidate@test.com",
		ResumeS3Key:    "k",
		ResumeFilename: "f",
	}
	require.NoError(t, appRepo.Create(ctx, app))

	t.Run("Create and GetByID / ListByApplicationID", func(t *testing.T) {
		note1 := &domain.ApplicationNote{
			ApplicationID: app.ID,
			AuthorName:    "Recruiter Alice",
			NoteText:      "Initial phone screen completed. Candidate seems strong.",
		}
		require.NoError(t, noteRepo.Create(ctx, note1))
		assert.NotEmpty(t, note1.ID)
		assert.False(t, note1.CreatedAt.IsZero())

		note2 := &domain.ApplicationNote{
			ApplicationID: app.ID,
			AuthorName:    "Hiring Manager Bob",
			NoteText:      "Technical interview passed with great feedback.",
		}
		require.NoError(t, noteRepo.Create(ctx, note2))

		// GetByID
		fetched, err := noteRepo.GetByID(ctx, note1.ID)
		require.NoError(t, err)
		assert.Equal(t, note1.ID, fetched.ID)
		assert.Equal(t, note1.AuthorName, fetched.AuthorName)
		assert.Equal(t, note1.NoteText, fetched.NoteText)

		// ListByApplicationID
		notes, err := noteRepo.ListByApplicationID(ctx, app.ID)
		require.NoError(t, err)
		assert.Len(t, notes, 2)
		assert.Equal(t, "Recruiter Alice", notes[0].AuthorName)
		assert.Equal(t, "Hiring Manager Bob", notes[1].AuthorName)
	})

	t.Run("Create note with non-existent application returns ErrNotFound", func(t *testing.T) {
		note := &domain.ApplicationNote{
			ApplicationID: "00000000-0000-0000-0000-000000000000",
			AuthorName:    "Alice",
			NoteText:      "Test note",
		}
		err := noteRepo.Create(ctx, note)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetByID not found", func(t *testing.T) {
		_, err := noteRepo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("Delete note and cascade delete on application delete", func(t *testing.T) {
		note := &domain.ApplicationNote{
			ApplicationID: app.ID,
			AuthorName:    "Alice",
			NoteText:      "Delete me",
		}
		require.NoError(t, noteRepo.Create(ctx, note))

		err := noteRepo.Delete(ctx, note.ID)
		require.NoError(t, err)

		_, err = noteRepo.GetByID(ctx, note.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = noteRepo.Delete(ctx, note.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Check cascade
		noteToCascade := &domain.ApplicationNote{
			ApplicationID: app.ID,
			AuthorName:    "Alice",
			NoteText:      "Cascade me",
		}
		require.NoError(t, noteRepo.Create(ctx, noteToCascade))

		require.NoError(t, appRepo.Delete(ctx, app.ID))

		_, err = noteRepo.GetByID(ctx, noteToCascade.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
