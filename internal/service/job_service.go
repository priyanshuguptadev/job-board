package service

import (
	"context"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// JobService defines business operations for jobs in public and ATS workflows.
type JobService interface {
	ListPublishedJobs(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error)
	GetPublishedJob(ctx context.Context, idOrSlug string) (*domain.Job, error)
	ListDepartments(ctx context.Context) ([]string, error)
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
