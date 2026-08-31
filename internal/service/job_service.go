package service

import (
	"context"
	"strings"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// CreateJobInput contains the input parameters for creating a new job listing.
type CreateJobInput struct {
	Slug                string
	Title               string
	Department          string
	Location            string
	EmploymentType      domain.EmploymentType
	ExperienceLevel     *string
	SalaryMin           *float64
	SalaryMax           *float64
	SalaryCurrency      string
	DescriptionMarkdown string
	Status              domain.JobStatus
	CustomFields        []domain.CustomField
}

// UpdateJobInput contains the input parameters for updating an existing job listing.
type UpdateJobInput struct {
	Slug                *string
	Title               *string
	Department          *string
	Location            *string
	EmploymentType      *domain.EmploymentType
	ExperienceLevel     *string
	SalaryMin           *float64
	SalaryMax           *float64
	SalaryCurrency      *string
	DescriptionMarkdown *string
	Status              *domain.JobStatus
	CustomFields        *[]domain.CustomField
}

// JobService defines business operations for jobs in public and ATS workflows.
type JobService interface {
	// Public
	ListPublishedJobs(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error)
	GetPublishedJob(ctx context.Context, idOrSlug string) (*domain.Job, error)
	ListDepartments(ctx context.Context) ([]string, error)

	// Admin
	CreateJob(ctx context.Context, input CreateJobInput) (*domain.Job, error)
	ListJobs(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error)
	GetJob(ctx context.Context, idOrSlug string) (*domain.Job, error)
	UpdateJob(ctx context.Context, id string, input UpdateJobInput) (*domain.Job, error)
	DeleteJob(ctx context.Context, id string) error
}

type jobService struct {
	repo domain.JobRepository
}

// NewJobService creates a new JobService instance.
func NewJobService(repo domain.JobRepository) JobService {
	if repo == nil {
		panic("service: job repo is required")
	}
	return &jobService{repo: repo}
}

// ListPublishedJobs returns paginated published job listings matching the filter.
func (s *jobService) ListPublishedJobs(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	filter.Status = domain.JobStatusPublished

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	jobs, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if jobs == nil {
		jobs = []*domain.Job{}
	}
	return jobs, total, nil
}

// GetPublishedJob returns a single published job by ID or slug.
// If the job does not exist or its status is not 'published', it returns domain.ErrNotFound.
func (s *jobService) GetPublishedJob(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	job, err := s.repo.GetByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	if job == nil || job.Status != domain.JobStatusPublished {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

// ListDepartments returns distinct department names with active published jobs.
func (s *jobService) ListDepartments(ctx context.Context) ([]string, error) {
	departments, err := s.repo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	if departments == nil {
		departments = []string{}
	}
	return departments, nil
}

// CreateJob creates a new job listing with domain validation and default values.
func (s *jobService) CreateJob(ctx context.Context, input CreateJobInput) (*domain.Job, error) {
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = domain.GenerateSlug(input.Title)
	}

	empType := input.EmploymentType
	if empType == "" {
		empType = domain.EmploymentTypeFullTime
	}

	status := input.Status
	if status == "" {
		status = domain.JobStatusDraft
	}

	currency := strings.TrimSpace(input.SalaryCurrency)
	if currency == "" {
		currency = "USD"
	}

	customFields := input.CustomFields
	if customFields == nil {
		customFields = []domain.CustomField{}
	}

	details := domain.ValidateJobInput(
		input.Title,
		input.Department,
		input.Location,
		input.DescriptionMarkdown,
		empType,
		status,
		input.ExperienceLevel,
		input.SalaryMin,
		input.SalaryMax,
		currency,
		slug,
		customFields,
	)
	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	var publishedAt *time.Time
	if status == domain.JobStatusPublished {
		now := time.Now().UTC()
		publishedAt = &now
	}

	job := &domain.Job{
		ID:                  domain.NewID(),
		Slug:                slug,
		Title:               strings.TrimSpace(input.Title),
		Department:          strings.TrimSpace(input.Department),
		Location:            strings.TrimSpace(input.Location),
		EmploymentType:      empType,
		ExperienceLevel:     input.ExperienceLevel,
		SalaryMin:           input.SalaryMin,
		SalaryMax:           input.SalaryMax,
		SalaryCurrency:      currency,
		DescriptionMarkdown: strings.TrimSpace(input.DescriptionMarkdown),
		Status:              status,
		CustomFields:        customFields,
		PublishedAt:         publishedAt,
	}

	if err := s.repo.Create(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

// ListJobs returns all jobs matching the filter with pagination.
func (s *jobService) ListJobs(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	jobs, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if jobs == nil {
		jobs = []*domain.Job{}
	}
	return jobs, total, nil
}

// GetJob retrieves any job by ID or slug.
func (s *jobService) GetJob(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	job, err := s.repo.GetByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

// UpdateJob performs partial updates on an existing job.
func (s *jobService) UpdateJob(ctx context.Context, id string, input UpdateJobInput) (*domain.Job, error) {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}

	updatedTitle := job.Title
	if input.Title != nil {
		updatedTitle = strings.TrimSpace(*input.Title)
	}

	updatedDept := job.Department
	if input.Department != nil {
		updatedDept = strings.TrimSpace(*input.Department)
	}

	updatedLoc := job.Location
	if input.Location != nil {
		updatedLoc = strings.TrimSpace(*input.Location)
	}

	updatedDesc := job.DescriptionMarkdown
	if input.DescriptionMarkdown != nil {
		updatedDesc = strings.TrimSpace(*input.DescriptionMarkdown)
	}

	updatedEmpType := job.EmploymentType
	if input.EmploymentType != nil {
		updatedEmpType = *input.EmploymentType
	}

	updatedStatus := job.Status
	if input.Status != nil {
		updatedStatus = *input.Status
	}

	updatedExp := job.ExperienceLevel
	if input.ExperienceLevel != nil {
		updatedExp = input.ExperienceLevel
	}

	updatedSalaryMin := job.SalaryMin
	if input.SalaryMin != nil {
		updatedSalaryMin = input.SalaryMin
	}

	updatedSalaryMax := job.SalaryMax
	if input.SalaryMax != nil {
		updatedSalaryMax = input.SalaryMax
	}

	updatedCurrency := job.SalaryCurrency
	if input.SalaryCurrency != nil {
		updatedCurrency = strings.TrimSpace(*input.SalaryCurrency)
	}

	updatedSlug := job.Slug
	if input.Slug != nil {
		updatedSlug = strings.TrimSpace(*input.Slug)
	}

	updatedCustomFields := job.CustomFields
	if input.CustomFields != nil {
		updatedCustomFields = *input.CustomFields
	}

	details := domain.ValidateJobInput(
		updatedTitle,
		updatedDept,
		updatedLoc,
		updatedDesc,
		updatedEmpType,
		updatedStatus,
		updatedExp,
		updatedSalaryMin,
		updatedSalaryMax,
		updatedCurrency,
		updatedSlug,
		updatedCustomFields,
	)
	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	job.Title = updatedTitle
	job.Department = updatedDept
	job.Location = updatedLoc
	job.DescriptionMarkdown = updatedDesc
	job.EmploymentType = updatedEmpType
	job.ExperienceLevel = updatedExp
	job.SalaryMin = updatedSalaryMin
	job.SalaryMax = updatedSalaryMax
	job.SalaryCurrency = updatedCurrency
	job.Slug = updatedSlug
	job.CustomFields = updatedCustomFields

	if input.Status != nil {
		if *input.Status == domain.JobStatusPublished && job.PublishedAt == nil {
			now := time.Now().UTC()
			job.PublishedAt = &now
		}
		job.Status = *input.Status
	}

	if err := s.repo.Update(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

// DeleteJob permanently deletes a job listing.
func (s *jobService) DeleteJob(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
