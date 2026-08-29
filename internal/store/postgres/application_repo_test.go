package postgres_test

import (
	"context"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/store/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationRepository(t *testing.T) {
	db := getTestDB(t)
	jobRepo := postgres.NewJobRepository(db)
	appRepo := postgres.NewApplicationRepository(db)
	ctx := context.Background()

	cleanTables(t, db)

	// Create test job
	job := &domain.Job{
		Slug:                "backend-engineer",
		Title:               "Backend Engineer",
		Department:          "Engineering",
		Location:            "Remote",
		EmploymentType:      domain.EmploymentTypeFullTime,
		DescriptionMarkdown: "We need a backend engineer.",
		Status:              domain.JobStatusPublished,
	}
	require.NoError(t, jobRepo.Create(ctx, job))

	t.Run("Create and GetByID", func(t *testing.T) {
		phone := "+1234567890"
		linkedin := "https://linkedin.com/in/janedoe"
		app := &domain.Application{
			JobID:          job.ID,
			CandidateName:  "Jane Doe",
			CandidateEmail: "jane@example.com",
			CandidatePhone: &phone,
			LinkedInURL:    &linkedin,
			ResumeS3Key:    "resumes/jane_doe.pdf",
			ResumeFilename: "jane_doe.pdf",
			CustomAnswers: map[string]interface{}{
				"years_exp":  float64(5),
				"github_url": "https://github.com/janedoe",
			},
			Stage: domain.ApplicationStageApplied,
		}

		err := appRepo.Create(ctx, app)
		require.NoError(t, err)
		assert.NotEmpty(t, app.ID)
		assert.False(t, app.CreatedAt.IsZero())
		assert.False(t, app.UpdatedAt.IsZero())

		fetched, err := appRepo.GetByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, app.ID, fetched.ID)
		assert.Equal(t, app.JobID, fetched.JobID)
		assert.Equal(t, "Jane Doe", fetched.CandidateName)
		assert.Equal(t, "jane@example.com", fetched.CandidateEmail)
		assert.Equal(t, phone, *fetched.CandidatePhone)
		assert.Equal(t, linkedin, *fetched.LinkedInURL)
		assert.Equal(t, "resumes/jane_doe.pdf", fetched.ResumeS3Key)
		assert.Equal(t, domain.ApplicationStageApplied, fetched.Stage)
		assert.Equal(t, float64(5), fetched.CustomAnswers["years_exp"])
		assert.Equal(t, "https://github.com/janedoe", fetched.CustomAnswers["github_url"])
	})

	t.Run("Create with invalid job_id returns ErrNotFound", func(t *testing.T) {
		app := &domain.Application{
			JobID:          "00000000-0000-0000-0000-000000000000",
			CandidateName:  "Invalid Job Candidate",
			CandidateEmail: "invalid@example.com",
			ResumeS3Key:    "resumes/test.pdf",
			ResumeFilename: "test.pdf",
		}
		err := appRepo.Create(ctx, app)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetByID not found", func(t *testing.T) {
		_, err := appRepo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("List with filtering and pagination", func(t *testing.T) {
		cleanTables(t, db)
		require.NoError(t, jobRepo.Create(ctx, job))

		job2 := &domain.Job{
			Slug:                "frontend-engineer",
			Title:               "Frontend Engineer",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "desc",
			Status:              domain.JobStatusPublished,
		}
		require.NoError(t, jobRepo.Create(ctx, job2))

		apps := []*domain.Application{
			{JobID: job.ID, CandidateName: "Alice", CandidateEmail: "alice@example.com", ResumeS3Key: "k1", ResumeFilename: "f1", Stage: domain.ApplicationStageApplied},
			{JobID: job.ID, CandidateName: "Bob", CandidateEmail: "bob@example.com", ResumeS3Key: "k2", ResumeFilename: "f2", Stage: domain.ApplicationStageInterviewing},
			{JobID: job.ID, CandidateName: "Charlie", CandidateEmail: "charlie@test.org", ResumeS3Key: "k3", ResumeFilename: "f3", Stage: domain.ApplicationStageOffer},
			{JobID: job2.ID, CandidateName: "David", CandidateEmail: "david@example.com", ResumeS3Key: "k4", ResumeFilename: "f4", Stage: domain.ApplicationStageApplied},
		}
		for _, a := range apps {
			require.NoError(t, appRepo.Create(ctx, a))
		}

		// Filter by JobID
		list, total, err := appRepo.List(ctx, domain.ApplicationListFilter{JobID: job.ID})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)

		// Filter by Stage
		list, total, err = appRepo.List(ctx, domain.ApplicationListFilter{Stage: domain.ApplicationStageApplied})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)

		// Filter by CandidateEmail substring
		list, total, err = appRepo.List(ctx, domain.ApplicationListFilter{CandidateEmail: "example.com"})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)

		// Pagination
		list, total, err = appRepo.List(ctx, domain.ApplicationListFilter{Page: 1, Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, list, 2)
	})

	t.Run("Update and UpdateStage", func(t *testing.T) {
		cleanTables(t, db)
		require.NoError(t, jobRepo.Create(ctx, job))

		app := &domain.Application{
			JobID:          job.ID,
			CandidateName:  "Candidate Update",
			CandidateEmail: "candidate@update.com",
			ResumeS3Key:    "k",
			ResumeFilename: "f",
			Stage:          domain.ApplicationStageApplied,
		}
		require.NoError(t, appRepo.Create(ctx, app))

		// Full update
		app.CandidateName = "Updated Candidate"
		app.CustomAnswers = map[string]interface{}{"note": "updated"}
		err := appRepo.Update(ctx, app)
		require.NoError(t, err)

		fetched, err := appRepo.GetByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Candidate", fetched.CandidateName)
		assert.Equal(t, "updated", fetched.CustomAnswers["note"])

		// UpdateStage
		reason := "Position filled by internal candidate"
		err = appRepo.UpdateStage(ctx, app.ID, domain.ApplicationStageRejected, &reason)
		require.NoError(t, err)

		fetched, err = appRepo.GetByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.ApplicationStageRejected, fetched.Stage)
		require.NotNil(t, fetched.RejectedReason)
		assert.Equal(t, reason, *fetched.RejectedReason)

		// UpdateStage not found
		err = appRepo.UpdateStage(ctx, "00000000-0000-0000-0000-000000000000", domain.ApplicationStageOffer, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("Delete and Cascade", func(t *testing.T) {
		cleanTables(t, db)
		require.NoError(t, jobRepo.Create(ctx, job))

		app := &domain.Application{
			JobID:          job.ID,
			CandidateName:  "To Delete",
			CandidateEmail: "todelete@example.com",
			ResumeS3Key:    "k",
			ResumeFilename: "f",
		}
		require.NoError(t, appRepo.Create(ctx, app))

		// Direct delete
		err := appRepo.Delete(ctx, app.ID)
		require.NoError(t, err)

		_, err = appRepo.GetByID(ctx, app.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = appRepo.Delete(ctx, app.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// Cascade delete on job delete
		app2 := &domain.Application{
			JobID:          job.ID,
			CandidateName:  "Cascade Test",
			CandidateEmail: "cascade@example.com",
			ResumeS3Key:    "k",
			ResumeFilename: "f",
		}
		require.NoError(t, appRepo.Create(ctx, app2))

		require.NoError(t, jobRepo.Delete(ctx, job.ID))

		_, err = appRepo.GetByID(ctx, app2.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
