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

func TestJobRepository(t *testing.T) {
	db := getTestDB(t)
	repo := postgres.NewJobRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByID / GetBySlug", func(t *testing.T) {
		minSalary := 120000.00
		maxSalary := 160000.00
		expLevel := "Senior"
		publishedAt := time.Now().Truncate(time.Microsecond)

		job := &domain.Job{
			Slug:                "senior-go-developer",
			Title:               "Senior Go Developer",
			Department:          "Engineering",
			Location:            "Remote, US",
			EmploymentType:      domain.EmploymentTypeFullTime,
			ExperienceLevel:     &expLevel,
			SalaryMin:           &minSalary,
			SalaryMax:           &maxSalary,
			SalaryCurrency:      "USD",
			DescriptionMarkdown: "### Role Overview\nWe are looking for a Senior Go Engineer.",
			Status:              domain.JobStatusPublished,
			CustomFields: []domain.CustomField{
				{
					ID:       "github_url",
					Label:    "GitHub Profile URL",
					Type:     "url",
					Required: true,
				},
				{
					ID:       "years_exp",
					Label:    "Years of Experience",
					Type:     "number",
					Required: true,
				},
			},
			PublishedAt: &publishedAt,
		}

		err := repo.Create(ctx, job)
		require.NoError(t, err)
		assert.NotEmpty(t, job.ID)
		assert.False(t, job.CreatedAt.IsZero())
		assert.False(t, job.UpdatedAt.IsZero())

		// GetByID
		fetchedByID, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.Slug, fetchedByID.Slug)
		assert.Equal(t, job.Title, fetchedByID.Title)
		assert.Equal(t, job.Department, fetchedByID.Department)
		assert.Equal(t, job.Location, fetchedByID.Location)
		assert.Equal(t, domain.EmploymentTypeFullTime, fetchedByID.EmploymentType)
		assert.Equal(t, "USD", fetchedByID.SalaryCurrency)
		assert.Equal(t, 2, len(fetchedByID.CustomFields))
		assert.Equal(t, "github_url", fetchedByID.CustomFields[0].ID)
		assert.True(t, fetchedByID.CustomFields[0].Required)

		// GetBySlug
		fetchedBySlug, err := repo.GetBySlug(ctx, job.Slug)
		require.NoError(t, err)
		assert.Equal(t, job.ID, fetchedBySlug.ID)

		// GetByIDOrSlug using UUID
		fetchedOrUUID, err := repo.GetByIDOrSlug(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, fetchedOrUUID.ID)

		// GetByIDOrSlug using slug
		fetchedOrSlug, err := repo.GetByIDOrSlug(ctx, job.Slug)
		require.NoError(t, err)
		assert.Equal(t, job.ID, fetchedOrSlug.ID)
	})

	t.Run("Create duplicate slug returns ErrConflict", func(t *testing.T) {
		job1 := &domain.Job{
			Slug:                "duplicate-slug-job",
			Title:               "Job 1",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "Desc 1",
		}
		require.NoError(t, repo.Create(ctx, job1))

		job2 := &domain.Job{
			Slug:                "duplicate-slug-job",
			Title:               "Job 2",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "Desc 2",
		}
		err := repo.Create(ctx, job2)
		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("Get not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrNotFound)

		_, err = repo.GetBySlug(ctx, "non-existent-slug-12345")
		assert.ErrorIs(t, err, domain.ErrNotFound)

		_, err = repo.GetByIDOrSlug(ctx, "non-existent-slug-12345")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("List with filtering and pagination", func(t *testing.T) {
		cleanTables(t, db)

		jobs := []*domain.Job{
			{Slug: "j1", Title: "Job 1", Department: "Engineering", Location: "San Francisco, CA", EmploymentType: domain.EmploymentTypeFullTime, Status: domain.JobStatusPublished, DescriptionMarkdown: "desc"},
			{Slug: "j2", Title: "Job 2", Department: "Engineering", Location: "New York, NY", EmploymentType: domain.EmploymentTypeContract, Status: domain.JobStatusPublished, DescriptionMarkdown: "desc"},
			{Slug: "j3", Title: "Job 3", Department: "Product", Location: "Remote", EmploymentType: domain.EmploymentTypeFullTime, Status: domain.JobStatusPublished, DescriptionMarkdown: "desc"},
			{Slug: "j4", Title: "Job 4", Department: "Design", Location: "Remote", EmploymentType: domain.EmploymentTypePartTime, Status: domain.JobStatusDraft, DescriptionMarkdown: "desc"},
			{Slug: "j5", Title: "Job 5", Department: "Engineering", Location: "Remote", EmploymentType: domain.EmploymentTypeFullTime, Status: domain.JobStatusArchived, DescriptionMarkdown: "desc"},
		}
		for _, j := range jobs {
			require.NoError(t, repo.Create(ctx, j))
		}

		// Filter by Department
		list, total, err := repo.List(ctx, domain.JobListFilter{Department: "Engineering"})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)

		// Filter by Location substring
		list, total, err = repo.List(ctx, domain.JobListFilter{Location: "remote"})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)

		// Filter by Status
		list, total, err = repo.List(ctx, domain.JobListFilter{Status: domain.JobStatusPublished})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)

		// Filter by EmploymentType & Status
		list, total, err = repo.List(ctx, domain.JobListFilter{EmploymentType: domain.EmploymentTypeFullTime, Status: domain.JobStatusPublished})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)

		// Pagination
		list, total, err = repo.List(ctx, domain.JobListFilter{Page: 1, Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, list, 2)

		listPage2, total2, err := repo.List(ctx, domain.JobListFilter{Page: 2, Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 5, total2)
		assert.Len(t, listPage2, 2)
		assert.NotEqual(t, list[0].ID, listPage2[0].ID)
	})

	t.Run("ListDepartments returns distinct published departments", func(t *testing.T) {
		cleanTables(t, db)

		require.NoError(t, repo.Create(ctx, &domain.Job{Slug: "j1", Title: "J1", Department: "Engineering", Location: "L", Status: domain.JobStatusPublished, DescriptionMarkdown: "d"}))
		require.NoError(t, repo.Create(ctx, &domain.Job{Slug: "j2", Title: "J2", Department: "Engineering", Location: "L", Status: domain.JobStatusPublished, DescriptionMarkdown: "d"}))
		require.NoError(t, repo.Create(ctx, &domain.Job{Slug: "j3", Title: "J3", Department: "Product", Location: "L", Status: domain.JobStatusPublished, DescriptionMarkdown: "d"}))
		require.NoError(t, repo.Create(ctx, &domain.Job{Slug: "j4", Title: "J4", Department: "Marketing", Location: "L", Status: domain.JobStatusDraft, DescriptionMarkdown: "d"}))

		depts, err := repo.ListDepartments(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"Engineering", "Product"}, depts)
	})

	t.Run("Update job", func(t *testing.T) {
		cleanTables(t, db)

		job := &domain.Job{
			Slug:                "initial-job",
			Title:               "Initial Title",
			Department:          "Engineering",
			Location:            "Remote",
			EmploymentType:      domain.EmploymentTypeFullTime,
			DescriptionMarkdown: "Initial Desc",
			Status:              domain.JobStatusDraft,
		}
		require.NoError(t, repo.Create(ctx, job))

		job.Title = "Updated Title"
		job.Status = domain.JobStatusPublished
		now := time.Now().Truncate(time.Microsecond)
		job.PublishedAt = &now
		job.CustomFields = []domain.CustomField{
			{ID: "portfolio", Label: "Portfolio", Type: "url", Required: false},
		}

		err := repo.Update(ctx, job)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", fetched.Title)
		assert.Equal(t, domain.JobStatusPublished, fetched.Status)
		assert.NotNil(t, fetched.PublishedAt)
		assert.Len(t, fetched.CustomFields, 1)

		// Update non-existent job
		job.ID = "00000000-0000-0000-0000-000000000000"
		job.Slug = "non-existent-update"
		err = repo.Update(ctx, job)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("Delete job", func(t *testing.T) {
		cleanTables(t, db)

		job := &domain.Job{
			Slug:                "job-to-delete",
			Title:               "Job",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "Desc",
		}
		require.NoError(t, repo.Create(ctx, job))

		err := repo.Delete(ctx, job.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, job.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		err = repo.Delete(ctx, job.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
