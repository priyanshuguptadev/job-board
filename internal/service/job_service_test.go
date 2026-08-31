package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockJobRepo struct {
	jobs         map[string]*domain.Job
	slugIndex    map[string]string
	departments  []string
	listErr      error
	getErr       error
	listDeptsErr error
}

func newMockJobRepo() *mockJobRepo {
	return &mockJobRepo{
		jobs:        make(map[string]*domain.Job),
		slugIndex:   make(map[string]string),
		departments: []string{"Engineering", "Product"},
	}
}

func (m *mockJobRepo) Create(_ context.Context, job *domain.Job) error {
	m.jobs[job.ID] = job
	m.slugIndex[job.Slug] = job.ID
	return nil
}

func (m *mockJobRepo) GetByID(_ context.Context, id string) (*domain.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

func (m *mockJobRepo) GetBySlug(_ context.Context, slug string) (*domain.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m.jobs[id], nil
}

func (m *mockJobRepo) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	if job, ok := m.jobs[idOrSlug]; ok {
		return job, nil
	}
	return m.GetBySlug(ctx, idOrSlug)
}

func (m *mockJobRepo) List(_ context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var res []*domain.Job
	for _, j := range m.jobs {
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		if filter.Department != "" && j.Department != filter.Department {
			continue
		}
		res = append(res, j)
	}
	return res, len(res), nil
}

func (m *mockJobRepo) ListDepartments(_ context.Context) ([]string, error) {
	if m.listDeptsErr != nil {
		return nil, m.listDeptsErr
	}
	return m.departments, nil
}

func (m *mockJobRepo) Update(_ context.Context, job *domain.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.jobs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.jobs, id)
	return nil
}

func TestJobService_ListPublishedJobs(t *testing.T) {
	repo := newMockJobRepo()
	svc := service.NewJobService(repo, nil)

	publishedJob := &domain.Job{
		ID:         "11111111-1111-1111-1111-111111111111",
		Slug:       "senior-go-engineer",
		Title:      "Senior Go Engineer",
		Department: "Engineering",
		Status:     domain.JobStatusPublished,
		CreatedAt:  time.Now(),
	}
	draftJob := &domain.Job{
		ID:         "22222222-2222-2222-2222-222222222222",
		Slug:       "draft-job",
		Title:      "Draft Job",
		Department: "Engineering",
		Status:     domain.JobStatusDraft,
		CreatedAt:  time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), publishedJob))
	require.NoError(t, repo.Create(context.Background(), draftJob))

	t.Run("lists only published jobs", func(t *testing.T) {
		jobs, total, err := svc.ListPublishedJobs(context.Background(), domain.JobListFilter{})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, jobs, 1)
		assert.Equal(t, publishedJob.ID, jobs[0].ID)
	})

	t.Run("filters by department", func(t *testing.T) {
		jobs, total, err := svc.ListPublishedJobs(context.Background(), domain.JobListFilter{Department: "Product"})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, jobs)
	})
}

func TestJobService_GetPublishedJob(t *testing.T) {
	repo := newMockJobRepo()
	svc := service.NewJobService(repo, nil)

	publishedJob := &domain.Job{
		ID:         "11111111-1111-1111-1111-111111111111",
		Slug:       "senior-go-engineer",
		Title:      "Senior Go Engineer",
		Department: "Engineering",
		Status:     domain.JobStatusPublished,
	}
	draftJob := &domain.Job{
		ID:         "22222222-2222-2222-2222-222222222222",
		Slug:       "draft-job",
		Title:      "Draft Job",
		Department: "Engineering",
		Status:     domain.JobStatusDraft,
	}
	require.NoError(t, repo.Create(context.Background(), publishedJob))
	require.NoError(t, repo.Create(context.Background(), draftJob))

	t.Run("get published job by slug", func(t *testing.T) {
		job, err := svc.GetPublishedJob(context.Background(), "senior-go-engineer")
		require.NoError(t, err)
		assert.Equal(t, publishedJob.ID, job.ID)
	})

	t.Run("get draft job returns not found", func(t *testing.T) {
		_, err := svc.GetPublishedJob(context.Background(), "draft-job")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("get non-existent job returns not found", func(t *testing.T) {
		_, err := svc.GetPublishedJob(context.Background(), "non-existent")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestJobService_ListDepartments(t *testing.T) {
	repo := newMockJobRepo()
	svc := service.NewJobService(repo, nil)

	depts, err := svc.ListDepartments(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"Engineering", "Product"}, depts)
}

func TestJobService_AdminCRUD(t *testing.T) {
	repo := newMockJobRepo()
	svc := service.NewJobService(repo, nil)

	t.Run("CreateJob validates and persists draft job", func(t *testing.T) {
		min := 90000.0
		max := 130000.0
		input := service.CreateJobInput{
			Title:               "Backend Engineer",
			Department:          "Engineering",
			Location:            "Remote",
			EmploymentType:      domain.EmploymentTypeFullTime,
			SalaryMin:           &min,
			SalaryMax:           &max,
			DescriptionMarkdown: "Markdown desc",
			Status:              domain.JobStatusDraft,
		}

		job, err := svc.CreateJob(context.Background(), input)
		require.NoError(t, err)
		assert.NotEmpty(t, job.ID)
		assert.Equal(t, "backend-engineer", job.Slug)
		assert.Equal(t, domain.JobStatusDraft, job.Status)
		assert.Nil(t, job.PublishedAt)
	})

	t.Run("CreateJob with published status sets PublishedAt", func(t *testing.T) {
		input := service.CreateJobInput{
			Title:               "Product Manager",
			Department:          "Product",
			Location:            "SF",
			EmploymentType:      domain.EmploymentTypeContract,
			DescriptionMarkdown: "PM role",
			Status:              domain.JobStatusPublished,
		}

		job, err := svc.CreateJob(context.Background(), input)
		require.NoError(t, err)
		assert.NotEmpty(t, job.ID)
		assert.Equal(t, domain.JobStatusPublished, job.Status)
		assert.NotNil(t, job.PublishedAt)
	})

	t.Run("CreateJob validation failure", func(t *testing.T) {
		input := service.CreateJobInput{
			Title: "",
		}

		_, err := svc.CreateJob(context.Background(), input)
		require.Error(t, err)
		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
	})

	t.Run("ListJobs returns all jobs regardless of status", func(t *testing.T) {
		jobs, total, err := svc.ListJobs(context.Background(), domain.JobListFilter{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.Len(t, jobs, total)
	})

	t.Run("UpdateJob updates fields and publishes", func(t *testing.T) {
		input := service.CreateJobInput{
			Title:               "QA Engineer",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "QA role",
			Status:              domain.JobStatusDraft,
		}
		job, err := svc.CreateJob(context.Background(), input)
		require.NoError(t, err)

		newTitle := "Senior QA Engineer"
		newStatus := domain.JobStatusPublished
		updated, err := svc.UpdateJob(context.Background(), job.ID, service.UpdateJobInput{
			Title:  &newTitle,
			Status: &newStatus,
		})
		require.NoError(t, err)
		assert.Equal(t, "Senior QA Engineer", updated.Title)
		assert.Equal(t, domain.JobStatusPublished, updated.Status)
		assert.NotNil(t, updated.PublishedAt)
	})

	t.Run("DeleteJob deletes job", func(t *testing.T) {
		input := service.CreateJobInput{
			Title:               "Temporary Role",
			Department:          "Ops",
			Location:            "NY",
			DescriptionMarkdown: "Temp role",
		}
		job, err := svc.CreateJob(context.Background(), input)
		require.NoError(t, err)

		err = svc.DeleteJob(context.Background(), job.ID)
		require.NoError(t, err)

		_, err = svc.GetJob(context.Background(), job.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestJobService_EventDispatching(t *testing.T) {
	repo := newMockJobRepo()
	mockWS := &mockEventDispatcher{}
	svc := service.NewJobService(repo, mockWS)

	t.Run("CreateJob with published status dispatches job.published", func(t *testing.T) {
		mockWS.events = nil
		_, err := svc.CreateJob(context.Background(), service.CreateJobInput{
			Title:               "Platform Engineer",
			Department:          "Engineering",
			Location:            "Remote",
			DescriptionMarkdown: "Platform role",
			Status:              domain.JobStatusPublished,
		})
		require.NoError(t, err)
		assert.Contains(t, mockWS.events, domain.EventJobPublished)
	})

	t.Run("UpdateJob to published dispatches job.published", func(t *testing.T) {
		mockWS.events = nil
		job, err := svc.CreateJob(context.Background(), service.CreateJobInput{
			Title:               "Data Engineer",
			Department:          "Data",
			Location:            "Remote",
			DescriptionMarkdown: "Data role",
			Status:              domain.JobStatusDraft,
		})
		require.NoError(t, err)
		assert.Empty(t, mockWS.events)

		pubStatus := domain.JobStatusPublished
		_, err = svc.UpdateJob(context.Background(), job.ID, service.UpdateJobInput{
			Status: &pubStatus,
		})
		require.NoError(t, err)
		assert.Contains(t, mockWS.events, domain.EventJobPublished)
	})

	t.Run("UpdateJob to archived dispatches job.archived", func(t *testing.T) {
		mockWS.events = nil
		job, err := svc.CreateJob(context.Background(), service.CreateJobInput{
			Title:               "Security Lead",
			Department:          "Security",
			Location:            "Remote",
			DescriptionMarkdown: "Security role",
			Status:              domain.JobStatusPublished,
		})
		require.NoError(t, err)
		mockWS.events = nil

		archivedStatus := domain.JobStatusArchived
		_, err = svc.UpdateJob(context.Background(), job.ID, service.UpdateJobInput{
			Status: &archivedStatus,
		})
		require.NoError(t, err)
		assert.Contains(t, mockWS.events, domain.EventJobArchived)
	})
}
