package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/storage"
)

// ApplyInput contains the payload required to submit a job application.
type ApplyInput struct {
	JobIDOrSlug             string
	CandidateName           string
	CandidateEmail          string
	CandidatePhone          *string
	LinkedInURL             *string
	ResumeFilename          string
	ResumeSize              int64
	ResumeReader            io.Reader
	ResumeHeaderContentType string
	CustomAnswers           map[string]interface{}
}

// UpdateStageInput contains stage update parameters.
type UpdateStageInput struct {
	Stage          domain.ApplicationStage
	RejectedReason *string
}

// CreateNoteInput contains review note parameters.
type CreateNoteInput struct {
	AuthorName string
	NoteText   string
}

// ResumeDownloadResult contains presigned download information for a candidate's resume.
type ResumeDownloadResult struct {
	URL              string `json:"url"`
	Filename         string `json:"filename"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// ApplicationService defines business operations for managing candidate applications.
type ApplicationService interface {
	// Public
	Apply(ctx context.Context, input ApplyInput) (*domain.Application, error)

	// Admin candidate & pipeline
	ListApplications(ctx context.Context, filter domain.ApplicationListFilter) ([]*domain.Application, int, error)
	GetApplication(ctx context.Context, id string) (*domain.Application, error)
	GetResumeDownloadURL(ctx context.Context, id string) (*ResumeDownloadResult, error)
	UpdateStage(ctx context.Context, id string, input UpdateStageInput) (*domain.Application, error)
	CreateNote(ctx context.Context, appID string, input CreateNoteInput) (*domain.ApplicationNote, error)
	ListNotes(ctx context.Context, appID string) ([]*domain.ApplicationNote, error)
}

type applicationService struct {
	jobRepo  domain.JobRepository
	appRepo  domain.ApplicationRepository
	noteRepo domain.ApplicationNoteRepository
	storage  storage.Storage
}

// NewApplicationService creates a new ApplicationService instance.
func NewApplicationService(jobRepo domain.JobRepository, appRepo domain.ApplicationRepository, noteRepo domain.ApplicationNoteRepository, strg storage.Storage) ApplicationService {
	return &applicationService{
		jobRepo:  jobRepo,
		appRepo:  appRepo,
		noteRepo: noteRepo,
		storage:  strg,
	}
}

// Apply orchestrates candidate application submission:
// 1. Verifies job is published.
// 2. Validates candidate and custom answer domain rules.
// 3. Validates and uploads resume to S3 storage.
// 4. Persists the application in the database.
func (s *applicationService) Apply(ctx context.Context, input ApplyInput) (*domain.Application, error) {
	// 1. Fetch published job
	job, err := s.jobRepo.GetByIDOrSlug(ctx, input.JobIDOrSlug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if job.Status != domain.JobStatusPublished {
		return nil, domain.ErrNotFound
	}

	// 2. Domain validations
	var details []domain.ErrorDetail

	candidateDetails := domain.ValidateCandidateInput(input.CandidateName, input.CandidateEmail, input.CandidatePhone, input.LinkedInURL)
	if len(candidateDetails) > 0 {
		details = append(details, candidateDetails...)
	}

	customAnswerDetails := domain.ValidateCustomAnswers(job.CustomFields, input.CustomAnswers)
	if len(customAnswerDetails) > 0 {
		details = append(details, customAnswerDetails...)
	}

	// 3. Check resume presence and upper bounds
	if input.ResumeReader == nil || input.ResumeFilename == "" || input.ResumeSize == 0 {
		details = append(details, domain.ErrorDetail{
			Field: "resume",
			Issue: "Resume file is required",
		})
	} else if input.ResumeSize > storage.MaxResumeSize {
		details = append(details, domain.ErrorDetail{
			Field: "resume",
			Issue: "File exceeds the maximum allowed size of 10MB",
		})
	}

	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	// 4. Validate resume format & content
	validationResult, err := storage.ValidateResume(input.ResumeFilename, input.ResumeSize, input.ResumeReader)
	if err != nil {
		if errors.Is(err, storage.ErrUnsupportedMediaType) || errors.Is(err, domain.ErrUnsupportedMediaType) {
			return nil, domain.ErrUnsupportedMediaType
		}
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: []domain.ErrorDetail{
				{
					Field: "resume",
					Issue: err.Error(),
				},
			},
		}
	}

	// 5. Generate application ID & S3 key
	appID := domain.NewID()
	resumeKey := storage.GenerateResumeKey(job.ID, appID, validationResult.SanitizedFilename)

	// 6. Upload resume attachment
	if s.storage != nil {
		if err := s.storage.Upload(ctx, resumeKey, validationResult.Reader, validationResult.Size, validationResult.ContentType); err != nil {
			return nil, fmt.Errorf("failed to upload resume: %w", err)
		}
	}

	// 7. Persist application
	customAnswers := input.CustomAnswers
	if customAnswers == nil {
		customAnswers = make(map[string]interface{})
	}

	app := &domain.Application{
		ID:             appID,
		JobID:          job.ID,
		CandidateName:  input.CandidateName,
		CandidateEmail: input.CandidateEmail,
		CandidatePhone: input.CandidatePhone,
		LinkedInURL:    input.LinkedInURL,
		ResumeS3Key:    resumeKey,
		ResumeFilename: validationResult.SanitizedFilename,
		CustomAnswers:  customAnswers,
		Stage:          domain.ApplicationStageApplied,
	}

	if err := s.appRepo.Create(ctx, app); err != nil {
		if s.storage != nil {
			_ = s.storage.Delete(ctx, resumeKey)
		}
		return nil, fmt.Errorf("failed to persist application: %w", err)
	}

	return app, nil
}

// ListApplications retrieves candidate applications matching the filter.
func (s *applicationService) ListApplications(ctx context.Context, filter domain.ApplicationListFilter) ([]*domain.Application, int, error) {
	if filter.JobID != "" {
		if _, err := s.jobRepo.GetByIDOrSlug(ctx, filter.JobID); err != nil {
			return nil, 0, err
		}
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	apps, total, err := s.appRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if apps == nil {
		apps = []*domain.Application{}
	}
	return apps, total, nil
}

// GetApplication retrieves an application by ID.
func (s *applicationService) GetApplication(ctx context.Context, id string) (*domain.Application, error) {
	app, err := s.appRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, domain.ErrNotFound
	}
	return app, nil
}

// GetResumeDownloadURL generates a presigned download URL for a candidate's resume attachment.
func (s *applicationService) GetResumeDownloadURL(ctx context.Context, id string) (*ResumeDownloadResult, error) {
	app, err := s.appRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, domain.ErrNotFound
	}

	if s.storage == nil {
		return nil, errors.New("storage client not configured")
	}

	expiry := 15 * time.Minute
	url, err := s.storage.GetPresignedURL(ctx, app.ResumeS3Key, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned url: %w", err)
	}

	return &ResumeDownloadResult{
		URL:              url,
		Filename:         app.ResumeFilename,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

// UpdateStage updates the hiring pipeline stage of an application.
func (s *applicationService) UpdateStage(ctx context.Context, id string, input UpdateStageInput) (*domain.Application, error) {
	app, err := s.appRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, domain.ErrNotFound
	}

	details := domain.ValidateStageUpdate(input.Stage, input.RejectedReason)
	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	if err := s.appRepo.UpdateStage(ctx, id, input.Stage, input.RejectedReason); err != nil {
		return nil, err
	}

	app.Stage = input.Stage
	app.RejectedReason = input.RejectedReason
	app.UpdatedAt = time.Now().UTC()

	return app, nil
}

// CreateNote adds an internal review note to a candidate application.
func (s *applicationService) CreateNote(ctx context.Context, appID string, input CreateNoteInput) (*domain.ApplicationNote, error) {
	if _, err := s.appRepo.GetByID(ctx, appID); err != nil {
		return nil, err
	}

	details := domain.ValidateNoteInput(input.AuthorName, input.NoteText)
	if len(details) > 0 {
		return nil, &domain.ValidationError{
			Message: "The provided input failed validation checks.",
			Details: details,
		}
	}

	note := &domain.ApplicationNote{
		ID:            domain.NewID(),
		ApplicationID: appID,
		AuthorName:    strings.TrimSpace(input.AuthorName),
		NoteText:      strings.TrimSpace(input.NoteText),
	}

	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, err
	}

	return note, nil
}

// ListNotes retrieves all internal review notes for an application.
func (s *applicationService) ListNotes(ctx context.Context, appID string) ([]*domain.ApplicationNote, error) {
	if _, err := s.appRepo.GetByID(ctx, appID); err != nil {
		return nil, err
	}

	notes, err := s.noteRepo.ListByApplicationID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if notes == nil {
		notes = []*domain.ApplicationNote{}
	}
	return notes, nil
}
