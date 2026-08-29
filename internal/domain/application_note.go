package domain

import (
	"context"
	"time"
)

// ApplicationNote represents an internal review note on a candidate's application.
type ApplicationNote struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	AuthorName    string    `json:"author_name"`
	NoteText      string    `json:"note_text"`
	CreatedAt     time.Time `json:"created_at"`
}

// ApplicationNoteRepository defines persistence operations for application review notes.
type ApplicationNoteRepository interface {
	Create(ctx context.Context, note *ApplicationNote) error
	GetByID(ctx context.Context, id string) (*ApplicationNote, error)
	ListByApplicationID(ctx context.Context, appID string) ([]*ApplicationNote, error)
	Delete(ctx context.Context, id string) error
}
