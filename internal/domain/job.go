package domain

import (
	"context"
	"time"
)

// JobStatus represents the lifecycle state of a job posting.
type JobStatus string

const (
	JobStatusDraft     JobStatus = "draft"
	JobStatusPublished JobStatus = "published"
	JobStatusArchived  JobStatus = "archived"
)

// EmploymentType represents the contract classification of a job.
type EmploymentType string

const (
	EmploymentTypeFullTime   EmploymentType = "full_time"
	EmploymentTypePartTime   EmploymentType = "part_time"
	EmploymentTypeContract   EmploymentType = "contract"
	EmploymentTypeInternship EmploymentType = "internship"
)

// CustomField defines a dynamic form field schema for a job application.
type CustomField struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // e.g. text, number, url, select, textarea, boolean
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}

// Job represents a job posting entity.
type Job struct {
	ID                  string         `json:"id"`
	Slug                string         `json:"slug"`
	Title               string         `json:"title"`
	Department          string         `json:"department"`
	Location            string         `json:"location"`
	EmploymentType      EmploymentType `json:"employment_type"`
	ExperienceLevel     *string        `json:"experience_level,omitempty"`
	SalaryMin           *float64       `json:"salary_min,omitempty"`
	SalaryMax           *float64       `json:"salary_max,omitempty"`
	SalaryCurrency      string         `json:"salary_currency"`
	DescriptionMarkdown string         `json:"description_markdown"`
	Status              JobStatus      `json:"status"`
	CustomFields        []CustomField  `json:"custom_fields"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	PublishedAt         *time.Time     `json:"published_at,omitempty"`
}

// JobListFilter defines filtering and pagination parameters for querying jobs.
type JobListFilter struct {
	Department     string
	Location       string
	EmploymentType EmploymentType
	Status         JobStatus
	Page           int
	Limit          int
}

// JobRepository defines persistence operations for jobs.
type JobRepository interface {
	Create(ctx context.Context, job *Job) error
	GetByID(ctx context.Context, id string) (*Job, error)
	GetBySlug(ctx context.Context, slug string) (*Job, error)
	GetByIDOrSlug(ctx context.Context, idOrSlug string) (*Job, error)
	List(ctx context.Context, filter JobListFilter) ([]*Job, int, error)
	ListDepartments(ctx context.Context) ([]string, error)
	Update(ctx context.Context, job *Job) error
	Delete(ctx context.Context, id string) error
}
