package service_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAppRepo struct {
	apps      map[string]*domain.Application
	createErr error
}

func newMockAppRepo() *mockAppRepo {
	return &mockAppRepo{
		apps: make(map[string]*domain.Application),
	}
}

func (m *mockAppRepo) Create(_ context.Context, app *domain.Application) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.apps[app.ID] = app
	return nil
}

func (m *mockAppRepo) GetByID(_ context.Context, id string) (*domain.Application, error) {
	app, ok := m.apps[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return app, nil
}

func (m *mockAppRepo) List(_ context.Context, _ domain.ApplicationListFilter) ([]*domain.Application, int, error) {
	var res []*domain.Application
	for _, a := range m.apps {
		res = append(res, a)
	}
	return res, len(res), nil
}

func (m *mockAppRepo) Update(_ context.Context, app *domain.Application) error {
	m.apps[app.ID] = app
	return nil
}

func (m *mockAppRepo) UpdateStage(_ context.Context, id string, stage domain.ApplicationStage, reason *string) error {
	app, ok := m.apps[id]
	if !ok {
		return domain.ErrNotFound
	}
	app.Stage = stage
	app.RejectedReason = reason
	return nil
}

func (m *mockAppRepo) Delete(_ context.Context, id string) error {
	delete(m.apps, id)
	return nil
}

func sampleValidPDF() []byte {
	return []byte("%PDF-1.4 sample pdf content for unit testing")
}

func TestApplicationService_Apply(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	job := &domain.Job{
		ID:         "11111111-1111-1111-1111-111111111111",
		Slug:       "backend-engineer",
		Title:      "Backend Engineer",
		Department: "Engineering",
		Status:     domain.JobStatusPublished,
		CustomFields: []domain.CustomField{
			{
				ID:       "years_exp",
				Label:    "Years of Go Experience",
				Type:     "number",
				Required: true,
			},
			{
				ID:       "github",
				Label:    "GitHub URL",
				Type:     "url",
				Required: false,
			},
			{
				ID:       "sponsorship",
				Label:    "Sponsorship Required",
				Type:     "select",
				Options:  []string{"Yes", "No"},
				Required: true,
			},
		},
		CreatedAt: time.Now(),
	}
	require.NoError(t, jobRepo.Create(context.Background(), job))

	svc := service.NewApplicationService(jobRepo, appRepo, strg)

	t.Run("successful application submission", func(t *testing.T) {
		pdfContent := sampleValidPDF()
		input := service.ApplyInput{
			JobIDOrSlug:             job.ID,
			CandidateName:           "Jane Doe",
			CandidateEmail:          "jane@example.com",
			ResumeFilename:          "resume.pdf",
			ResumeSize:              int64(len(pdfContent)),
			ResumeReader:            bytes.NewReader(pdfContent),
			ResumeHeaderContentType: "application/pdf",
			CustomAnswers: map[string]interface{}{
				"years_exp":   5,
				"github":      "https://github.com/janedoe",
				"sponsorship": "No",
			},
		}

		app, err := svc.Apply(context.Background(), input)
		require.NoError(t, err)
		assert.NotEmpty(t, app.ID)
		assert.Equal(t, job.ID, app.JobID)
		assert.Equal(t, "Jane Doe", app.CandidateName)
		assert.Equal(t, "jane@example.com", app.CandidateEmail)
		assert.Equal(t, "resume.pdf", app.ResumeFilename)
		assert.Equal(t, domain.ApplicationStageApplied, app.Stage)

		// Verify stored in memory storage
		exists, err := strg.Exists(context.Background(), app.ResumeS3Key)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("fails when required candidate name or email is missing", func(t *testing.T) {
		pdfContent := sampleValidPDF()
		input := service.ApplyInput{
			JobIDOrSlug:    job.ID,
			CandidateName:  "",
			CandidateEmail: "not-an-email",
			ResumeFilename: "resume.pdf",
			ResumeSize:     int64(len(pdfContent)),
			ResumeReader:   bytes.NewReader(pdfContent),
		}

		_, err := svc.Apply(context.Background(), input)
		require.Error(t, err)
		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Len(t, valErr.Details, 4) // candidate_name, candidate_email format, years_exp, sponsorship
	})

	t.Run("fails when custom field answer is invalid", func(t *testing.T) {
		pdfContent := sampleValidPDF()
		input := service.ApplyInput{
			JobIDOrSlug:    job.ID,
			CandidateName:  "Jane Doe",
			CandidateEmail: "jane@example.com",
			ResumeFilename: "resume.pdf",
			ResumeSize:     int64(len(pdfContent)),
			ResumeReader:   bytes.NewReader(pdfContent),
			CustomAnswers: map[string]interface{}{
				"years_exp":   "not-a-number",
				"github":      "not-a-url",
				"sponsorship": "Maybe", // not in options
			},
		}

		_, err := svc.Apply(context.Background(), input)
		require.Error(t, err)
		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Len(t, valErr.Details, 3)
	})

	t.Run("fails with unsupported media type for executable or plain text", func(t *testing.T) {
		textData := []byte("plain text not a pdf")
		input := service.ApplyInput{
			JobIDOrSlug:    job.ID,
			CandidateName:  "Jane Doe",
			CandidateEmail: "jane@example.com",
			ResumeFilename: "resume.exe",
			ResumeSize:     int64(len(textData)),
			ResumeReader:   bytes.NewReader(textData),
			CustomAnswers: map[string]interface{}{
				"years_exp":   3,
				"sponsorship": "No",
			},
		}

		_, err := svc.Apply(context.Background(), input)
		assert.ErrorIs(t, err, domain.ErrUnsupportedMediaType)
	})

	t.Run("fails when resume exceeds 10MB", func(t *testing.T) {
		input := service.ApplyInput{
			JobIDOrSlug:    job.ID,
			CandidateName:  "Jane Doe",
			CandidateEmail: "jane@example.com",
			ResumeFilename: "resume.pdf",
			ResumeSize:     11 * 1024 * 1024,
			ResumeReader:   strings.NewReader("dummy"),
			CustomAnswers: map[string]interface{}{
				"years_exp":   3,
				"sponsorship": "No",
			},
		}

		_, err := svc.Apply(context.Background(), input)
		require.Error(t, err)
		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "resume", valErr.Details[0].Field)
	})

	t.Run("returns not found for draft or non-existent job", func(t *testing.T) {
		pdfContent := sampleValidPDF()
		input := service.ApplyInput{
			JobIDOrSlug:    "non-existent-job",
			CandidateName:  "Jane Doe",
			CandidateEmail: "jane@example.com",
			ResumeFilename: "resume.pdf",
			ResumeSize:     int64(len(pdfContent)),
			ResumeReader:   bytes.NewReader(pdfContent),
		}

		_, err := svc.Apply(context.Background(), input)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
