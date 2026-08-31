package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
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

type mockJobRepo struct {
	jobs        map[string]*domain.Job
	slugIndex   map[string]string
	departments []string
}

func newMockJobRepo() *mockJobRepo {
	return &mockJobRepo{
		jobs:        make(map[string]*domain.Job),
		slugIndex:   make(map[string]string),
		departments: []string{"Engineering", "Product"},
	}
}

func (m *mockJobRepo) Create(_ context.Context, job *domain.Job) error {
	m.jobs[job.ID] = job
	m.slugIndex[job.Slug] = job.ID
	return nil
}

func (m *mockJobRepo) GetByID(_ context.Context, id string) (*domain.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

func (m *mockJobRepo) GetBySlug(_ context.Context, slug string) (*domain.Job, error) {
	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m.jobs[id], nil
}

func (m *mockJobRepo) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	if job, ok := m.jobs[idOrSlug]; ok {
		return job, nil
	}
	return m.GetBySlug(ctx, idOrSlug)
}

func (m *mockJobRepo) List(_ context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	var res []*domain.Job
	for _, j := range m.jobs {
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		if filter.Department != "" && j.Department != filter.Department {
			continue
		}
		if filter.Location != "" && !strings.Contains(strings.ToLower(j.Location), strings.ToLower(filter.Location)) {
			continue
		}
		if filter.EmploymentType != "" && j.EmploymentType != filter.EmploymentType {
			continue
		}
		res = append(res, j)
	}
	return res, len(res), nil
}

func (m *mockJobRepo) ListDepartments(_ context.Context) ([]string, error) {
	return m.departments, nil
}

func (m *mockJobRepo) Update(_ context.Context, job *domain.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.jobs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.jobs, id)
	return nil
}

type mockAppRepo struct {
	apps map[string]*domain.Application
}

func newMockAppRepo() *mockAppRepo {
	return &mockAppRepo{
		apps: make(map[string]*domain.Application),
	}
}

func (m *mockAppRepo) Create(_ context.Context, app *domain.Application) error {
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

type mockNoteRepo struct {
	notes map[string][]*domain.ApplicationNote
}

func newMockNoteRepo() *mockNoteRepo {
	return &mockNoteRepo{
		notes: make(map[string][]*domain.ApplicationNote),
	}
}

func (m *mockNoteRepo) Create(_ context.Context, note *domain.ApplicationNote) error {
	m.notes[note.ApplicationID] = append(m.notes[note.ApplicationID], note)
	return nil
}

func (m *mockNoteRepo) GetByID(_ context.Context, id string) (*domain.ApplicationNote, error) {
	for _, notes := range m.notes {
		for _, n := range notes {
			if n.ID == id {
				return n, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockNoteRepo) ListByApplicationID(_ context.Context, appID string) ([]*domain.ApplicationNote, error) {
	return m.notes[appID], nil
}

func (m *mockNoteRepo) Delete(_ context.Context, id string) error {
	for appID, notes := range m.notes {
		for i, n := range notes {
			if n.ID == id {
				m.notes[appID] = append(notes[:i], notes[i+1:]...)
				return nil
			}
		}
	}
	return domain.ErrNotFound
}

func setupPublicTestRouter(jobRepo domain.JobRepository, appRepo domain.ApplicationRepository, strg storage.Storage) http.Handler {
	jobSvc := service.NewJobService(jobRepo)
	appSvc := service.NewApplicationService(jobRepo, appRepo, newMockNoteRepo(), strg)
	h := v1.NewPublicHandler(jobSvc, appSvc, nil)

	r := chi.NewRouter()
	r.Route("/v1/public", func(public chi.Router) {
		public.Get("/jobs", h.ListJobs)
		public.Get("/jobs/{slug_or_id}", h.GetJob)
		public.Get("/departments", h.ListDepartments)
		public.Post("/jobs/{job_id}/apply", h.Apply)
	})
	return r
}

func sampleValidPDF() []byte {
	return []byte("%PDF-1.4 sample pdf content for unit testing")
}

func TestPublicHandler_ListJobs(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	pubJob1 := &domain.Job{
		ID:             "11111111-1111-1111-1111-111111111111",
		Slug:           "senior-backend-engineer",
		Title:          "Senior Backend Engineer",
		Department:     "Engineering",
		Location:       "Remote",
		EmploymentType: domain.EmploymentTypeFullTime,
		Status:         domain.JobStatusPublished,
		CreatedAt:      time.Now(),
	}
	pubJob2 := &domain.Job{
		ID:             "22222222-2222-2222-2222-222222222222",
		Slug:           "product-designer",
		Title:          "Product Designer",
		Department:     "Design",
		Location:       "San Francisco",
		EmploymentType: domain.EmploymentTypeContract,
		Status:         domain.JobStatusPublished,
		CreatedAt:      time.Now(),
	}
	draftJob := &domain.Job{
		ID:             "33333333-3333-3333-3333-333333333333",
		Slug:           "draft-position",
		Title:          "Draft Position",
		Department:     "Engineering",
		Location:       "Remote",
		EmploymentType: domain.EmploymentTypeFullTime,
		Status:         domain.JobStatusDraft,
		CreatedAt:      time.Now(),
	}

	require.NoError(t, jobRepo.Create(context.Background(), pubJob1))
	require.NoError(t, jobRepo.Create(context.Background(), pubJob2))
	require.NoError(t, jobRepo.Create(context.Background(), draftJob))

	router := setupPublicTestRouter(jobRepo, appRepo, strg)

	t.Run("lists all published jobs with pagination metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp struct {
			Data       []domain.Job       `json:"data"`
			Pagination api.PaginationMeta `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 2, resp.Pagination.TotalItems)
		assert.Equal(t, 1, resp.Pagination.TotalPages)
		assert.Len(t, resp.Data, 2)
	})

	t.Run("filters jobs by department", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs?department=Engineering", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp struct {
			Data       []domain.Job       `json:"data"`
			Pagination api.PaginationMeta `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Pagination.TotalItems)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "senior-backend-engineer", resp.Data[0].Slug)
	})

	t.Run("filters jobs by location", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs?location=Remote", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp struct {
			Data       []domain.Job       `json:"data"`
			Pagination api.PaginationMeta `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Pagination.TotalItems)
		assert.Equal(t, "senior-backend-engineer", resp.Data[0].Slug)
	})

	t.Run("filters jobs by employment_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs?employment_type=contract", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp struct {
			Data       []domain.Job       `json:"data"`
			Pagination api.PaginationMeta `json:"pagination"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Pagination.TotalItems)
		assert.Equal(t, "product-designer", resp.Data[0].Slug)
	})

	t.Run("returns 422 for invalid employment_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs?employment_type=temporary", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		require.Len(t, errResp.Error.Details, 1)
		assert.Equal(t, "employment_type", errResp.Error.Details[0].Field)
	})

	t.Run("returns 422 for invalid page query param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs?page=abc", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		assert.Equal(t, "page", errResp.Error.Details[0].Field)
	})
}

func TestPublicHandler_GetJob(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	pubJob := &domain.Job{
		ID:             "11111111-1111-1111-1111-111111111111",
		Slug:           "engineering-lead",
		Title:          "Engineering Lead",
		Department:     "Engineering",
		Location:       "Remote",
		EmploymentType: domain.EmploymentTypeFullTime,
		Status:         domain.JobStatusPublished,
		CustomFields: []domain.CustomField{
			{
				ID:       "years_exp",
				Label:    "Years of experience",
				Type:     "number",
				Required: true,
			},
		},
		CreatedAt: time.Now(),
	}
	draftJob := &domain.Job{
		ID:         "22222222-2222-2222-2222-222222222222",
		Slug:       "draft-eng-lead",
		Title:      "Draft Eng Lead",
		Department: "Engineering",
		Status:     domain.JobStatusDraft,
		CreatedAt:  time.Now(),
	}

	require.NoError(t, jobRepo.Create(context.Background(), pubJob))
	require.NoError(t, jobRepo.Create(context.Background(), draftJob))

	router := setupPublicTestRouter(jobRepo, appRepo, strg)

	t.Run("retrieves published job by slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs/engineering-lead", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var job domain.Job
		err := json.Unmarshal(rec.Body.Bytes(), &job)
		require.NoError(t, err)
		assert.Equal(t, pubJob.ID, job.ID)
		assert.Equal(t, "Engineering Lead", job.Title)
		require.Len(t, job.CustomFields, 1)
		assert.Equal(t, "years_exp", job.CustomFields[0].ID)
	})

	t.Run("retrieves published job by ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs/11111111-1111-1111-1111-111111111111", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var job domain.Job
		err := json.Unmarshal(rec.Body.Bytes(), &job)
		require.NoError(t, err)
		assert.Equal(t, pubJob.ID, job.ID)
	})

	t.Run("returns 404 for draft job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs/draft-eng-lead", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeNotFound, errResp.Error.Code)
	})

	t.Run("returns 404 for non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs/non-existent-slug", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeNotFound, errResp.Error.Code)
	})
}

func TestPublicHandler_ListDepartments(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	router := setupPublicTestRouter(jobRepo, appRepo, strg)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/departments", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Departments []string `json:"departments"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, []string{"Engineering", "Product"}, resp.Departments)
}

func createMultipartRequest(url string, fields map[string]string, filename string, fileContent []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, err
		}
	}

	if filename != "" {
		part, err := writer.CreateFormFile("resume", filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(fileContent); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestPublicHandler_Apply(t *testing.T) {
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	job := &domain.Job{
		ID:         "11111111-1111-1111-1111-111111111111",
		Slug:       "lead-go-architect",
		Title:      "Lead Go Architect",
		Department: "Engineering",
		Status:     domain.JobStatusPublished,
		CustomFields: []domain.CustomField{
			{
				ID:       "years_exp",
				Label:    "Years of Go experience",
				Type:     "number",
				Required: true,
			},
			{
				ID:       "sponsorship",
				Label:    "Do you require visa sponsorship?",
				Type:     "select",
				Options:  []string{"Yes", "No"},
				Required: true,
			},
		},
		CreatedAt: time.Now(),
	}
	require.NoError(t, jobRepo.Create(context.Background(), job))

	router := setupPublicTestRouter(jobRepo, appRepo, strg)

	t.Run("successfully submits application with 201 Created and Location header", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "Alice Smith",
			"candidate_email": "alice@example.com",
			"candidate_phone": "+1-555-0199",
			"linkedin_url":    "https://linkedin.com/in/alicesmith",
			"custom_answers":  `{"years_exp": 7, "sponsorship": "No"}`,
		}
		pdf := sampleValidPDF()

		req, err := createMultipartRequest("/v1/public/jobs/"+job.ID+"/apply", fields, "alice_resume.pdf", pdf)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Location"))

		var resp v1.ApplicationSubmissionResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, job.ID, resp.JobID)
		assert.Equal(t, "Alice Smith", resp.CandidateName)
		assert.Equal(t, "alice@example.com", resp.CandidateEmail)
		assert.Equal(t, "alice_resume.pdf", resp.ResumeFilename)
		assert.Equal(t, domain.ApplicationStageApplied, resp.Stage)
		assert.Equal(t, "Application submitted successfully", resp.Message)
		assert.Equal(t, fmt.Sprintf("/v1/admin/applications/%s", resp.ID), rec.Header().Get("Location"))

		// Check app exists in repo
		app, err := appRepo.GetByID(context.Background(), resp.ID)
		require.NoError(t, err)
		assert.Equal(t, "Alice Smith", app.CandidateName)

		// Check resume exists in storage
		exists, err := strg.Exists(context.Background(), app.ResumeS3Key)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("submits application using job slug in path", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "Bob Martin",
			"candidate_email": "bob@example.com",
			"custom_answers":  `{"years_exp": 4, "sponsorship": "Yes"}`,
		}
		pdf := sampleValidPDF()

		req, err := createMultipartRequest("/v1/public/jobs/"+job.Slug+"/apply", fields, "bob_resume.pdf", pdf)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("returns 422 for missing required candidate fields and custom answers", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "",
			"candidate_email": "invalid-email-address",
		}
		pdf := sampleValidPDF()

		req, err := createMultipartRequest("/v1/public/jobs/"+job.ID+"/apply", fields, "resume.pdf", pdf)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		assert.NotEmpty(t, errResp.Error.Details)
	})

	t.Run("returns 422 for invalid custom answers JSON format", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "Charlie",
			"candidate_email": "charlie@example.com",
			"custom_answers":  `{bad json}`,
		}
		pdf := sampleValidPDF()

		req, err := createMultipartRequest("/v1/public/jobs/"+job.ID+"/apply", fields, "resume.pdf", pdf)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errResp api.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeValidationError, errResp.Error.Code)
		assert.Equal(t, "custom_answers", errResp.Error.Details[0].Field)
	})

	t.Run("returns 415 for unsupported resume format (e.g. .exe)", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "Dave",
			"candidate_email": "dave@example.com",
			"custom_answers":  `{"years_exp": 3, "sponsorship": "No"}`,
		}
		exeBytes := []byte("MZ this is an executable")

		req, err := createMultipartRequest("/v1/public/jobs/"+job.ID+"/apply", fields, "resume.exe", exeBytes)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		var errResp api.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeUnsupportedMediaType, errResp.Error.Code)
	})

	t.Run("returns 404 for non-existent job in apply", func(t *testing.T) {
		fields := map[string]string{
			"candidate_name":  "Eve",
			"candidate_email": "eve@example.com",
		}
		pdf := sampleValidPDF()

		req, err := createMultipartRequest("/v1/public/jobs/non-existent-job-id/apply", fields, "resume.pdf", pdf)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var errResp api.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeNotFound, errResp.Error.Code)
	})
}
