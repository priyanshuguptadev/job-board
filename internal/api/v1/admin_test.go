package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	api "github.com/priyanshuguptadev/job-board/internal/api/httputil"
	v1 "github.com/priyanshuguptadev/job-board/internal/api/v1"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminTestRouter(jobRepo domain.JobRepository, appRepo domain.ApplicationRepository, noteRepo domain.ApplicationNoteRepository, strg storage.Storage) http.Handler {
	jobSvc := service.NewJobService(jobRepo)
	appSvc := service.NewApplicationService(jobRepo, appRepo, noteRepo, strg)
	h := v1.NewAdminHandler(jobSvc, appSvc, nil)

	r := chi.NewRouter()
	r.Route("/v1/admin", func(admin chi.Router) {
		admin.Post("/jobs", h.CreateJob)
		admin.Get("/jobs", h.ListJobs)
		admin.Get("/jobs/{id}", h.GetJob)
		admin.Patch("/jobs/{id}", h.UpdateJob)
		admin.Delete("/jobs/{id}", h.DeleteJob)

		admin.Get("/jobs/{id}/applications", h.ListJobApplications)
		admin.Get("/applications/{id}", h.GetApplication)
		admin.Get("/applications/{id}/resume", h.GetResumeURL)
		admin.Patch("/applications/{id}/stage", h.UpdateStage)
		admin.Post("/applications/{id}/notes", h.CreateNote)
		admin.Get("/applications/{id}/notes", h.ListNotes)
	})
	return r
}

func TestAdminHandler_JobManagement(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	noteRepo := newMockNoteRepo()
	strg := storage.NewMemoryStorage()

	router := setupAdminTestRouter(jobRepo, appRepo, noteRepo, strg)

	t.Run("POST /v1/admin/jobs creates job successfully", func(t *testing.T) {
		min := 110000.0
		max := 160000.0
		reqBody := v1.CreateJobRequest{
			Title:               "Senior Backend Engineer",
			Department:          "Engineering",
			Location:            "Remote",
			EmploymentType:      domain.EmploymentTypeFullTime,
			SalaryMin:           &min,
			SalaryMax:           &max,
			SalaryCurrency:      "USD",
			DescriptionMarkdown: "## Senior Backend Engineer",
			Status:              domain.JobStatusDraft,
			CustomFields: []domain.CustomField{
				{
					ID:       "years_exp",
					Label:    "Years of experience",
					Type:     "number",
					Required: true,
				},
			},
		}

		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "/v1/admin/jobs/")

		var job domain.Job
		err = json.Unmarshal(rec.Body.Bytes(), &job)
		require.NoError(t, err)

		assert.NotEmpty(t, job.ID)
		assert.Equal(t, "senior-backend-engineer", job.Slug)
		assert.Equal(t, "Senior Backend Engineer", job.Title)
		assert.Equal(t, domain.JobStatusDraft, job.Status)
		require.Len(t, job.CustomFields, 1)
	})

	t.Run("POST /v1/admin/jobs returns 422 on validation error", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"title": "",
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		assert.NotEmpty(t, errResp.Error.Details)
	})

	t.Run("POST /v1/admin/jobs returns 422 on invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", strings.NewReader("{invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
	})

	t.Run("GET /v1/admin/jobs lists jobs with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs?page=1&limit=10", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Data       []domain.Job       `json:"data"`
			Pagination api.PaginationMeta `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, resp.Pagination.TotalItems, 1)
		assert.NotEmpty(t, resp.Data)
	})

	t.Run("GET /v1/admin/jobs returns 422 for invalid query params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs?status=invalid_status", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
	})

	t.Run("GET /v1/admin/jobs/{id} retrieves full job details", func(t *testing.T) {
		job := &domain.Job{
			ID:                  "job-100",
			Slug:                "lead-devops",
			Title:               "Lead DevOps",
			Department:          "Infrastructure",
			Location:            "Remote",
			EmploymentType:      domain.EmploymentTypeFullTime,
			DescriptionMarkdown: "Devops role",
			Status:              domain.JobStatusDraft,
			CreatedAt:           time.Now(),
		}
		require.NoError(t, jobRepo.Create(context.Background(), job))

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/job-100", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var got domain.Job
		err := json.Unmarshal(rec.Body.Bytes(), &got)
		require.NoError(t, err)
		assert.Equal(t, "job-100", got.ID)
		assert.Equal(t, "Lead DevOps", got.Title)
	})

	t.Run("GET /v1/admin/jobs/{id} returns 404 for non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/non-existent-id", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeNotFound, errResp.Error.Code)
	})

	t.Run("PATCH /v1/admin/jobs/{id} updates fields and publishes job", func(t *testing.T) {
		job := &domain.Job{
			ID:                  "job-200",
			Slug:                "frontend-engineer",
			Title:               "Frontend Engineer",
			Department:          "Engineering",
			Location:            "Austin, TX",
			EmploymentType:      domain.EmploymentTypeFullTime,
			DescriptionMarkdown: "Frontend react developer",
			Status:              domain.JobStatusDraft,
			CreatedAt:           time.Now(),
		}
		require.NoError(t, jobRepo.Create(context.Background(), job))

		newTitle := "Staff Frontend Engineer"
		newStatus := domain.JobStatusPublished
		reqBody := v1.UpdateJobRequest{
			Title:  &newTitle,
			Status: &newStatus,
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/jobs/job-200", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var updated domain.Job
		err = json.Unmarshal(rec.Body.Bytes(), &updated)
		require.NoError(t, err)

		assert.Equal(t, "Staff Frontend Engineer", updated.Title)
		assert.Equal(t, domain.JobStatusPublished, updated.Status)
		assert.NotNil(t, updated.PublishedAt)
	})

	t.Run("PATCH /v1/admin/jobs/{id} returns 404 for non-existent job", func(t *testing.T) {
		newTitle := "New Title"
		reqBody := v1.UpdateJobRequest{Title: &newTitle}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/jobs/non-existent-id", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("DELETE /v1/admin/jobs/{id} returns 204 No Content", func(t *testing.T) {
		job := &domain.Job{
			ID:                  "job-300",
			Slug:                "temporary-job",
			Title:               "Temporary Job",
			Department:          "HR",
			Location:            "Remote",
			EmploymentType:      domain.EmploymentTypeContract,
			DescriptionMarkdown: "Temp HR",
			Status:              domain.JobStatusDraft,
			CreatedAt:           time.Now(),
		}
		require.NoError(t, jobRepo.Create(context.Background(), job))

		req := httptest.NewRequest(http.MethodDelete, "/v1/admin/jobs/job-300", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Check deleted
		_, err := jobRepo.GetByID(context.Background(), "job-300")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("DELETE /v1/admin/jobs/{id} returns 404 for non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/admin/jobs/non-existent-id", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAdminHandler_CandidatePipelineAndNotes(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	noteRepo := newMockNoteRepo()
	strg := storage.NewMemoryStorage()

	job := &domain.Job{
		ID:                  "job-400",
		Slug:                "security-architect",
		Title:               "Security Architect",
		Department:          "Security",
		Location:            "Remote",
		EmploymentType:      domain.EmploymentTypeFullTime,
		DescriptionMarkdown: "Security role",
		Status:              domain.JobStatusPublished,
		CreatedAt:           time.Now(),
	}
	require.NoError(t, jobRepo.Create(context.Background(), job))

	app1 := &domain.Application{
		ID:             "app-100",
		JobID:          job.ID,
		CandidateName:  "Carol Danvers",
		CandidateEmail: "carol@example.com",
		ResumeS3Key:    "resumes/job-400/app-100/resume.pdf",
		ResumeFilename: "carol_resume.pdf",
		Stage:          domain.ApplicationStageApplied,
		CustomAnswers:  map[string]interface{}{"years_exp": 10},
		CreatedAt:      time.Now(),
	}
	require.NoError(t, appRepo.Create(context.Background(), app1))
	require.NoError(t, strg.Upload(context.Background(), app1.ResumeS3Key, bytes.NewReader(sampleValidPDF()), int64(len(sampleValidPDF())), "application/pdf"))

	router := setupAdminTestRouter(jobRepo, appRepo, noteRepo, strg)

	t.Run("GET /v1/admin/jobs/{id}/applications lists candidate applications", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/job-400/applications", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Data       []domain.Application `json:"data"`
			Pagination api.PaginationMeta   `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Pagination.TotalItems)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "Carol Danvers", resp.Data[0].CandidateName)
		assert.Equal(t, "carol_resume.pdf", resp.Data[0].ResumeFilename)
	})

	t.Run("GET /v1/admin/jobs/{id}/applications returns 404 for non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/non-existent-job/applications", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("GET /v1/admin/applications/{id} gets full candidate details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/applications/app-100", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var app domain.Application
		err := json.Unmarshal(rec.Body.Bytes(), &app)
		require.NoError(t, err)

		assert.Equal(t, "app-100", app.ID)
		assert.Equal(t, "Carol Danvers", app.CandidateName)
		assert.Equal(t, domain.ApplicationStageApplied, app.Stage)
	})

	t.Run("GET /v1/admin/applications/{id} returns 404 for non-existent application", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/applications/non-existent-app", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("GET /v1/admin/applications/{id}/resume generates presigned download URL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/applications/app-100/resume", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var res service.ResumeDownloadResult
		err := json.Unmarshal(rec.Body.Bytes(), &res)
		require.NoError(t, err)

		assert.NotEmpty(t, res.URL)
		assert.Equal(t, "carol_resume.pdf", res.Filename)
		assert.Equal(t, int64(900), res.ExpiresInSeconds)
	})

	t.Run("PATCH /v1/admin/applications/{id}/stage updates candidate hiring stage", func(t *testing.T) {
		reqBody := v1.UpdateStageRequest{
			Stage: domain.ApplicationStageInterviewing,
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/applications/app-100/stage", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var updated domain.Application
		err = json.Unmarshal(rec.Body.Bytes(), &updated)
		require.NoError(t, err)

		assert.Equal(t, domain.ApplicationStageInterviewing, updated.Stage)
	})

	t.Run("PATCH /v1/admin/applications/{id}/stage with rejection reason", func(t *testing.T) {
		reason := "Lacked required distributed systems experience"
		reqBody := v1.UpdateStageRequest{
			Stage:          domain.ApplicationStageRejected,
			RejectedReason: &reason,
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/applications/app-100/stage", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var updated domain.Application
		err = json.Unmarshal(rec.Body.Bytes(), &updated)
		require.NoError(t, err)

		assert.Equal(t, domain.ApplicationStageRejected, updated.Stage)
		assert.NotNil(t, updated.RejectedReason)
		assert.Equal(t, reason, *updated.RejectedReason)
	})

	t.Run("PATCH /v1/admin/applications/{id}/stage returns 422 on invalid stage", func(t *testing.T) {
		reqBody := map[string]string{
			"stage": "not_a_valid_stage",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/applications/app-100/stage", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		assert.Equal(t, "stage", errResp.Error.Details[0].Field)
	})

	t.Run("POST /v1/admin/applications/{id}/notes adds internal review note", func(t *testing.T) {
		reqBody := v1.CreateNoteRequest{
			AuthorName: "Lead Interviewer",
			NoteText:   "Passed architectural interview with high score.",
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/applications/app-100/notes", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "/v1/admin/applications/app-100/notes/")

		var note domain.ApplicationNote
		err = json.Unmarshal(rec.Body.Bytes(), &note)
		require.NoError(t, err)

		assert.NotEmpty(t, note.ID)
		assert.Equal(t, "app-100", note.ApplicationID)
		assert.Equal(t, "Lead Interviewer", note.AuthorName)
		assert.Equal(t, "Passed architectural interview with high score.", note.NoteText)
	})

	t.Run("POST /v1/admin/applications/{id}/notes returns 422 on validation failure", func(t *testing.T) {
		reqBody := v1.CreateNoteRequest{
			AuthorName: "",
			NoteText:   "",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/applications/app-100/notes", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("GET /v1/admin/applications/{id}/notes retrieves review notes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/applications/app-100/notes", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var notes []domain.ApplicationNote
		err := json.Unmarshal(rec.Body.Bytes(), &notes)
		require.NoError(t, err)

		require.Len(t, notes, 1)
		assert.Equal(t, "Lead Interviewer", notes[0].AuthorName)
	})

	t.Run("GET /v1/admin/applications/{id}/notes returns 404 for non-existent application", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/applications/non-existent-app/notes", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
