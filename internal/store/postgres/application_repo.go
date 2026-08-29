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

type applicationRepository struct {
	db *sql.DB
}

// NewApplicationRepository creates a new PostgreSQL Application repository.
func NewApplicationRepository(db *sql.DB) domain.ApplicationRepository {
	return &applicationRepository{db: db}
}

func (r *applicationRepository) Create(ctx context.Context, app *domain.Application) error {
	query := `
		INSERT INTO applications (
			id, job_id, candidate_name, candidate_email,
			candidate_phone, linkedin_url, resume_s3_key,
			resume_filename, custom_answers, stage,
			rejected_reason, created_at, updated_at
		)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`

	answersJSON, err := json.Marshal(app.CustomAnswers)
	if err != nil {
		return fmt.Errorf("failed to marshal custom answers: %w", err)
	}
	if string(answersJSON) == "null" {
		answersJSON = []byte("{}")
	}

	stage := app.Stage
	if stage == "" {
		stage = domain.ApplicationStageApplied
	}

	var id string
	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(
		ctx,
		query,
		app.ID,
		app.JobID,
		app.CandidateName,
		app.CandidateEmail,
		app.CandidatePhone,
		app.LinkedInURL,
		app.ResumeS3Key,
		app.ResumeFilename,
		answersJSON,
		stage,
		app.RejectedReason,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return mapDBError(fmt.Errorf("failed to create application: %w", err))
	}

	app.ID = id
	app.Stage = stage
	app.CreatedAt = createdAt
	app.UpdatedAt = updatedAt
	return nil
}

func (r *applicationRepository) scanApplication(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.Application, error) {
	var app domain.Application
	var answersJSON []byte

	err := scanner.Scan(
		&app.ID,
		&app.JobID,
		&app.CandidateName,
		&app.CandidateEmail,
		&app.CandidatePhone,
		&app.LinkedInURL,
		&app.ResumeS3Key,
		&app.ResumeFilename,
		&answersJSON,
		&app.Stage,
		&app.RejectedReason,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if len(answersJSON) > 0 {
		if err := json.Unmarshal(answersJSON, &app.CustomAnswers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom answers: %w", err)
		}
	} else {
		app.CustomAnswers = make(map[string]interface{})
	}

	return &app, nil
}

func (r *applicationRepository) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	query := `
		SELECT
			id, job_id, candidate_name, candidate_email,
			candidate_phone, linkedin_url, resume_s3_key,
			resume_filename, custom_answers, stage,
			rejected_reason, created_at, updated_at
		FROM applications
		WHERE id = $1
	`

	app, err := r.scanApplication(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get application by id: %w", err)
	}

	return app, nil
}

func (r *applicationRepository) List(ctx context.Context, filter domain.ApplicationListFilter) ([]*domain.Application, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.JobID != "" {
		conditions = append(conditions, fmt.Sprintf("job_id = $%d", argIdx))
		args = append(args, filter.JobID)
		argIdx++
	}

	if filter.Stage != "" {
		conditions = append(conditions, fmt.Sprintf("stage = $%d", argIdx))
		args = append(args, filter.Stage)
		argIdx++
	}

	if filter.CandidateEmail != "" {
		conditions = append(conditions, fmt.Sprintf("candidate_email ILIKE $%d", argIdx))
		args = append(args, "%"+filter.CandidateEmail+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total items
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM applications %s", whereClause)
	var totalItems int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		return nil, 0, fmt.Errorf("failed to count applications: %w", err)
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
			id, job_id, candidate_name, candidate_email,
			candidate_phone, linkedin_url, resume_s3_key,
			resume_filename, custom_answers, stage,
			rejected_reason, created_at, updated_at
		FROM applications
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	dataArgs := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query applications list: %w", err)
	}
	defer rows.Close()

	var applications []*domain.Application
	for rows.Next() {
		app, err := r.scanApplication(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan application: %w", err)
		}
		applications = append(applications, app)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return applications, totalItems, nil
}

func (r *applicationRepository) Update(ctx context.Context, app *domain.Application) error {
	answersJSON, err := json.Marshal(app.CustomAnswers)
	if err != nil {
		return fmt.Errorf("failed to marshal custom answers: %w", err)
	}
	if string(answersJSON) == "null" {
		answersJSON = []byte("{}")
	}

	query := `
		UPDATE applications
		SET
			candidate_name = $1,
			candidate_email = $2,
			candidate_phone = $3,
			linkedin_url = $4,
			resume_s3_key = $5,
			resume_filename = $6,
			custom_answers = $7,
			stage = $8,
			rejected_reason = $9,
			updated_at = NOW()
		WHERE id = $10
		RETURNING updated_at
	`

	var updatedAt time.Time
	err = r.db.QueryRowContext(
		ctx,
		query,
		app.CandidateName,
		app.CandidateEmail,
		app.CandidatePhone,
		app.LinkedInURL,
		app.ResumeS3Key,
		app.ResumeFilename,
		answersJSON,
		app.Stage,
		app.RejectedReason,
		app.ID,
	).Scan(&updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return mapDBError(fmt.Errorf("failed to update application: %w", err))
	}

	app.UpdatedAt = updatedAt
	return nil
}

func (r *applicationRepository) UpdateStage(ctx context.Context, id string, stage domain.ApplicationStage, rejectedReason *string) error {
	query := `
		UPDATE applications
		SET
			stage = $1,
			rejected_reason = $2,
			updated_at = NOW()
		WHERE id = $3
	`

	res, err := r.db.ExecContext(ctx, query, stage, rejectedReason, id)
	if err != nil {
		return fmt.Errorf("failed to update application stage: %w", err)
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

func (r *applicationRepository) Delete(ctx context.Context, id string) error {
	query := `
		DELETE FROM applications
		WHERE id = $1
	`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
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
