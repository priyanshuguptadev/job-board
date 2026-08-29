package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

type applicationNoteRepository struct {
	db *sql.DB
}

// NewApplicationNoteRepository creates a new PostgreSQL Application Note repository.
func NewApplicationNoteRepository(db *sql.DB) domain.ApplicationNoteRepository {
	return &applicationNoteRepository{db: db}
}

func (r *applicationNoteRepository) Create(ctx context.Context, note *domain.ApplicationNote) error {
	query := `
		INSERT INTO application_notes (
			id, application_id, author_name, note_text, created_at
		)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4, NOW()
		)
		RETURNING id, created_at
	`

	var id string
	var createdAt time.Time
	err := r.db.QueryRowContext(
		ctx,
		query,
		note.ID,
		note.ApplicationID,
		note.AuthorName,
		note.NoteText,
	).Scan(&id, &createdAt)

	if err != nil {
		return mapDBError(fmt.Errorf("failed to create application note: %w", err))
	}

	note.ID = id
	note.CreatedAt = createdAt
	return nil
}

func (r *applicationNoteRepository) GetByID(ctx context.Context, id string) (*domain.ApplicationNote, error) {
	query := `
		SELECT id, application_id, author_name, note_text, created_at
		FROM application_notes
		WHERE id = $1
	`

	var note domain.ApplicationNote
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&note.ID,
		&note.ApplicationID,
		&note.AuthorName,
		&note.NoteText,
		&note.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get application note by id: %w", err)
	}

	return &note, nil
}

func (r *applicationNoteRepository) ListByApplicationID(ctx context.Context, appID string) ([]*domain.ApplicationNote, error) {
	query := `
		SELECT id, application_id, author_name, note_text, created_at
		FROM application_notes
		WHERE application_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to list application notes: %w", err)
	}
	defer rows.Close()

	var notes []*domain.ApplicationNote
	for rows.Next() {
		var note domain.ApplicationNote
		if err := rows.Scan(
			&note.ID,
			&note.ApplicationID,
			&note.AuthorName,
			&note.NoteText,
			&note.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan application note: %w", err)
		}
		notes = append(notes, &note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return notes, nil
}

func (r *applicationNoteRepository) Delete(ctx context.Context, id string) error {
	query := `
		DELETE FROM application_notes
		WHERE id = $1
	`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete application note: %w", err)
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
