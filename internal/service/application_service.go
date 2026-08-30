package service

import (
	"context"
	"errors"
	"fmt"
	"io"

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

// ApplicationService defines business operations for managing candidate applications.
type ApplicationService interface {
	Apply(ctx context.Context, input ApplyInput) (*domain.Application, error)
}

type applicationService struct {
	jobRepo domain.JobRepository
	appRepo domain.ApplicationRepository
	storage storage.Storage
}

// NewApplicationService creates a new ApplicationService instance.
func NewApplicationService(jobRepo domain.JobRepository, appRepo domain.ApplicationRepository, strg storage.Storage) ApplicationService {
	return &applicationService{
		jobRepo: jobRepo,
		appRepo: appRepo,
		storage: strg,
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
