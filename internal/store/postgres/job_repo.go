package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

type jobRepository struct {
	db *sql.DB
}

// NewJobRepository creates a new PostgreSQL Job repository.
func NewJobRepository(db *sql.DB) domain.JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (
			id, slug, title, department, location, employment_type,
			experience_level, salary_min, salary_max, salary_currency,
			description_markdown, status, custom_fields,
			created_at, updated_at, published_at
		)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			NOW(), NOW(), $14
		)
		RETURNING id, created_at, updated_at
	`

	customFieldsJSON, err := json.Marshal(job.CustomFields)
	if err != nil {
		return fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	if string(customFieldsJSON) == "null" {
		customFieldsJSON = []byte("[]")
	}

	currency := job.SalaryCurrency
	if currency == "" {
		currency = "USD"
	}

	status := job.Status
	if status == "" {
		status = domain.JobStatusDraft
	}

	empType := job.EmploymentType
	if empType == "" {
		empType = domain.EmploymentTypeFullTime
	}

	var id string
	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.Slug,
		job.Title,
		job.Department,
		job.Location,
		empType,
		job.ExperienceLevel,
		job.SalaryMin,
		job.SalaryMax,
		currency,
		job.DescriptionMarkdown,
		status,
		customFieldsJSON,
		job.PublishedAt,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return mapDBError(fmt.Errorf("failed to create job: %w", err))
	}

	job.ID = id
	job.EmploymentType = empType
	job.SalaryCurrency = currency
	job.Status = status
	job.CreatedAt = createdAt
	job.UpdatedAt = updatedAt
	return nil
}

func (r *jobRepository) scanJob(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.Job, error) {
	var job domain.Job
	var customFieldsJSON []byte

	err := scanner.Scan(
		&job.ID,
		&job.Slug,
		&job.Title,
		&job.Department,
		&job.Location,
		&job.EmploymentType,
		&job.ExperienceLevel,
		&job.SalaryMin,
		&job.SalaryMax,
		&job.SalaryCurrency,
		&job.DescriptionMarkdown,
		&job.Status,
		&customFieldsJSON,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.PublishedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if len(customFieldsJSON) > 0 {
		if err := json.Unmarshal(customFieldsJSON, &job.CustomFields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom fields: %w", err)
		}
	} else {
		job.CustomFields = []domain.CustomField{}
	}

	return &job, nil
}

func (r *jobRepository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	query := `
		SELECT
			id, slug, title, department, location, employment_type,
			experience_level, salary_min, salary_max, salary_currency,
			description_markdown, status, custom_fields,
			created_at, updated_at, published_at
		FROM jobs
		WHERE id = $1
	`

	job, err := r.scanJob(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get job by id: %w", err)
	}

	return job, nil
}

func (r *jobRepository) GetBySlug(ctx context.Context, slug string) (*domain.Job, error) {
	query := `
		SELECT
			id, slug, title, department, location, employment_type,
			experience_level, salary_min, salary_max, salary_currency,
			description_markdown, status, custom_fields,
			created_at, updated_at, published_at
		FROM jobs
		WHERE slug = $1
	`

	job, err := r.scanJob(r.db.QueryRowContext(ctx, query, slug))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get job by slug: %w", err)
	}

	return job, nil
}

func (r *jobRepository) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	if isUUID(idOrSlug) {
		job, err := r.GetByID(ctx, idOrSlug)
		if err == nil {
			return job, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	return r.GetBySlug(ctx, idOrSlug)
}

func (r *jobRepository) List(ctx context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Department != "" {
		conditions = append(conditions, fmt.Sprintf("department = $%d", argIdx))
		args = append(args, filter.Department)
		argIdx++
	}

	if filter.Location != "" {
		conditions = append(conditions, fmt.Sprintf("location ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Location+"%")
		argIdx++
	}

	if filter.EmploymentType != "" {
		conditions = append(conditions, fmt.Sprintf("employment_type = $%d", argIdx))
		args = append(args, filter.EmploymentType)
		argIdx++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total items
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM jobs %s", whereClause)
	var totalItems int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	// Pagination limits
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	dataQuery := fmt.Sprintf(`
		SELECT
			id, slug, title, department, location, employment_type,
			experience_level, salary_min, salary_max, salary_currency,
			description_markdown, status, custom_fields,
			created_at, updated_at, published_at
		FROM jobs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	dataArgs := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query jobs list: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job, err := r.scanJob(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return jobs, totalItems, nil
}

func (r *jobRepository) ListDepartments(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT department
		FROM jobs
		WHERE status = 'published'
		ORDER BY department ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}
	defer rows.Close()

	var departments []string
	for rows.Next() {
		var dept string
		if err := rows.Scan(&dept); err != nil {
			return nil, fmt.Errorf("failed to scan department: %w", err)
		}
		departments = append(departments, dept)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return departments, nil
}

func (r *jobRepository) Update(ctx context.Context, job *domain.Job) error {
	customFieldsJSON, err := json.Marshal(job.CustomFields)
	if err != nil {
		return fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	if string(customFieldsJSON) == "null" {
		customFieldsJSON = []byte("[]")
	}

	query := `
		UPDATE jobs
		SET
			slug = $1,
			title = $2,
			department = $3,
			location = $4,
			employment_type = $5,
			experience_level = $6,
			salary_min = $7,
			salary_max = $8,
			salary_currency = $9,
			description_markdown = $10,
			status = $11,
			custom_fields = $12,
			updated_at = NOW(),
			published_at = $13
		WHERE id = $14
		RETURNING updated_at
	`

	var updatedAt time.Time
	err = r.db.QueryRowContext(
		ctx,
		query,
		job.Slug,
		job.Title,
		job.Department,
		job.Location,
		job.EmploymentType,
		job.ExperienceLevel,
		job.SalaryMin,
		job.SalaryMax,
		job.SalaryCurrency,
		job.DescriptionMarkdown,
		job.Status,
		customFieldsJSON,
		job.PublishedAt,
		job.ID,
	).Scan(&updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return mapDBError(fmt.Errorf("failed to update job: %w", err))
	}

	job.UpdatedAt = updatedAt
	return nil
}

func (r *jobRepository) Delete(ctx context.Context, id string) error {
	query := `
		DELETE FROM jobs
		WHERE id = $1
	`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}
