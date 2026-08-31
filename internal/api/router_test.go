package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/api"
	"github.com/priyanshuguptadev/job-board/internal/auth"
	"github.com/priyanshuguptadev/job-board/internal/config"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/storage"
	"github.com/priyanshuguptadev/job-board/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockApiKeyRepo struct {
	mu         sync.Mutex
	keysByHash map[string]*domain.ApiKey
}

func newMockApiKeyRepo() *mockApiKeyRepo {
	return &mockApiKeyRepo{
		keysByHash: make(map[string]*domain.ApiKey),
	}
}

func (m *mockApiKeyRepo) Create(_ context.Context, key *domain.ApiKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keysByHash[key.KeyHash] = key
	return nil
}

func (m *mockApiKeyRepo) GetByHash(_ context.Context, hash string) (*domain.ApiKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := m.keysByHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return key, nil
}

func (m *mockApiKeyRepo) GetByID(_ context.Context, _ string) (*domain.ApiKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApiKeyRepo) List(_ context.Context) ([]*domain.ApiKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApiKeyRepo) UpdateLastUsed(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (m *mockApiKeyRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type mockJobRepo struct {
	mu          sync.Mutex
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	m.slugIndex[job.Slug] = job.ID
	return nil
}

func (m *mockJobRepo) GetByID(_ context.Context, id string) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return job, nil
}

func (m *mockJobRepo) GetBySlug(_ context.Context, slug string) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m.jobs[id], nil
}

func (m *mockJobRepo) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Job, error) {
	if job, err := m.GetByID(ctx, idOrSlug); err == nil {
		return job, nil
	}
	return m.GetBySlug(ctx, idOrSlug)
}

func (m *mockJobRepo) List(_ context.Context, filter domain.JobListFilter) ([]*domain.Job, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.departments, nil
}

func (m *mockJobRepo) Update(_ context.Context, job *domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.jobs, id)
	return nil
}

type mockAppRepo struct {
	mu   sync.Mutex
	apps map[string]*domain.Application
}

func newMockAppRepo() *mockAppRepo {
	return &mockAppRepo{
		apps: make(map[string]*domain.Application),
	}
}

func (m *mockAppRepo) Create(_ context.Context, app *domain.Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apps[app.ID] = app
	return nil
}

func (m *mockAppRepo) GetByID(_ context.Context, id string) (*domain.Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	app, ok := m.apps[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return app, nil
}

func (m *mockAppRepo) List(_ context.Context, _ domain.ApplicationListFilter) ([]*domain.Application, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*domain.Application
	for _, a := range m.apps {
		res = append(res, a)
	}
	return res, len(res), nil
}

func (m *mockAppRepo) Update(_ context.Context, app *domain.Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apps[app.ID] = app
	return nil
}

func (m *mockAppRepo) UpdateStage(_ context.Context, id string, stage domain.ApplicationStage, reason *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	app, ok := m.apps[id]
	if !ok {
		return domain.ErrNotFound
	}
	app.Stage = stage
	app.RejectedReason = reason
	return nil
}

func (m *mockAppRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.apps, id)
	return nil
}

type mockNoteRepo struct {
	mu    sync.Mutex
	notes map[string][]*domain.ApplicationNote
}

func newMockNoteRepo() *mockNoteRepo {
	return &mockNoteRepo{
		notes: make(map[string][]*domain.ApplicationNote),
	}
}

func (m *mockNoteRepo) Create(_ context.Context, note *domain.ApplicationNote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes[note.ApplicationID] = append(m.notes[note.ApplicationID], note)
	return nil
}

func (m *mockNoteRepo) GetByID(_ context.Context, id string) (*domain.ApplicationNote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notes[appID], nil
}

func (m *mockNoteRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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

type mockWebhookRepo struct {
	mu   sync.Mutex
	subs map[string]*domain.WebhookSubscription
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{
		subs: make(map[string]*domain.WebhookSubscription),
	}
}

func (m *mockWebhookRepo) Create(_ context.Context, sub *domain.WebhookSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockWebhookRepo) GetByID(_ context.Context, id string) (*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return sub, nil
}

func (m *mockWebhookRepo) List(_ context.Context) ([]*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockWebhookRepo) ListActiveByEvent(_ context.Context, event string) ([]*domain.WebhookSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*domain.WebhookSubscription
	for _, s := range m.subs {
		if !s.IsActive {
			continue
		}
		for _, e := range s.Events {
			if e == "*" || e == event {
				list = append(list, s)
				break
			}
		}
	}
	return list, nil
}

func (m *mockWebhookRepo) Update(_ context.Context, sub *domain.WebhookSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockWebhookRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.subs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.subs, id)
	return nil
}

func newTestRouter(db *sql.DB, apiKeyRepo domain.ApiKeyRepository, rps, burst int) http.Handler {
	return newTestRouterWithRepos(db, apiKeyRepo, nil, nil, nil, rps, burst)
}

func newTestRouterWithRepos(db *sql.DB, apiKeyRepo domain.ApiKeyRepository, jobRepo domain.JobRepository, appRepo domain.ApplicationRepository, strg storage.Storage, rps, burst int) http.Handler {
	if rps <= 0 {
		rps = 20
	}
	if burst <= 0 {
		burst = 50
	}
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			Env:                "test",
			LogLevel:           "error",
			CORSAllowedOrigins: []string{"*"},
			RateLimitRPS:       rps,
			RateLimitBurst:     burst,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	webhookRepo := newMockWebhookRepo()
	webhookService := service.NewWebhookService(webhookRepo, nil)

	var jobService service.JobService
	var appService service.ApplicationService
	if jobRepo != nil {
		jobService = service.NewJobService(jobRepo, webhookService)
		if appRepo != nil {
			if strg == nil {
				strg = storage.NewMemoryStorage()
			}
			appService = service.NewApplicationService(jobRepo, appRepo, newMockNoteRepo(), strg, webhookService)
		}
	}

	return api.NewRouter(api.RouterConfig{
		Config:         cfg,
		Logger:         logger,
		DB:             db,
		ApiKeyRepo:     apiKeyRepo,
		JobService:     jobService,
		AppService:     appService,
		WebhookService: webhookService,
	})
}

func TestHealthCheck(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "disabled", body["database"])
}

func TestOpenAPISpecAndDocs(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

	t.Run("GET /openapi.json returns valid OpenAPI 3.0 spec", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

		var spec map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &spec)
		require.NoError(t, err)
		assert.Equal(t, "3.0.3", spec["openapi"])

		info, ok := spec["info"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Job Board API", info["title"])
	})

	t.Run("GET /docs returns Swagger UI HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Body.String(), "<title>Job Board API Documentation</title>")
		assert.Contains(t, rec.Body.String(), "/openapi.json")
	})

	t.Run("GET /docs/index.html returns Swagger UI HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/index.html", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Body.String(), "<title>Job Board API Documentation</title>")
	})
}

func TestNotFoundHandler(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeNotFound, resp.Error.Code)
	assert.NotEmpty(t, resp.Error.Message)
}

func TestMethodNotAllowedHandler(t *testing.T) {
	router := newTestRouter(nil, nil, 0, 0)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeMethodNotAllowed, resp.Error.Code)
}

func TestRouterAuthAndScoping(t *testing.T) {
	repo := newMockApiKeyRepo()

	adminRaw, adminKey, err := auth.GenerateKey("Admin", domain.ApiKeyScopeAdmin)
	require.NoError(t, err)
	adminKey.ID = "admin-1"
	require.NoError(t, repo.Create(context.Background(), adminKey))

	pubRaw, pubKey, err := auth.GenerateKey("Public", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-1"
	require.NoError(t, repo.Create(context.Background(), pubKey))

	router := newTestRouter(nil, repo, 20, 50)

	t.Run("public ping with valid public key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("public ping with admin key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin ping with admin key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin ping with public key returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var resp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeForbidden, resp.Error.Code)
	})

	t.Run("admin ping without key returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var resp api.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, api.ErrCodeInvalidAPIKey, resp.Error.Code)
	})
}

func TestRouterRateLimiting(t *testing.T) {
	repo := newMockApiKeyRepo()
	pubRaw, pubKey, err := auth.GenerateKey("Public", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-1"
	require.NoError(t, repo.Create(context.Background(), pubKey))

	// Burst of 2, 1 RPS
	router := newTestRouter(nil, repo, 1, 2)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 3rd request should hit rate limit
	req := httptest.NewRequest(http.MethodGet, "/v1/public/ping", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-API-Key", pubRaw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	var resp api.ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeRateLimitExceeded, resp.Error.Code)
}

func TestRespondErrorWithDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	api.RespondError(rec, http.StatusUnprocessableEntity, api.ErrCodeValidationError, "Validation failed", api.ErrorDetail{
		Field: "resume",
		Issue: "File exceeds 10MB limit",
	})

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	var resp api.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, api.ErrCodeValidationError, resp.Error.Code)
	assert.Equal(t, "Validation failed", resp.Error.Message)
	require.Len(t, resp.Error.Details, 1)
	assert.Equal(t, "resume", resp.Error.Details[0].Field)
	assert.Equal(t, "File exceeds 10MB limit", resp.Error.Details[0].Issue)
}

func TestRouterPublicCareerEndpoints(t *testing.T) {
	apiKeyRepo := newMockApiKeyRepo()
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	pubRaw, pubKey, err := auth.GenerateKey("Public", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-1"
	require.NoError(t, apiKeyRepo.Create(context.Background(), pubKey))

	job := &domain.Job{
		ID:                  "11111111-1111-1111-1111-111111111111",
		Slug:                "backend-developer",
		Title:               "Backend Developer",
		Department:          "Engineering",
		Location:            "Remote",
		EmploymentType:      domain.EmploymentTypeFullTime,
		DescriptionMarkdown: "We are looking for a Go developer.",
		Status:              domain.JobStatusPublished,
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
	require.NoError(t, jobRepo.Create(context.Background(), job))

	router := newTestRouterWithRepos(nil, apiKeyRepo, jobRepo, appRepo, strg, 20, 50)

	t.Run("GET /v1/public/jobs returns list of published jobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp api.PaginatedResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Pagination.TotalItems)
	})

	t.Run("GET /v1/public/jobs/{slug_or_id} returns job details", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/jobs/backend-developer", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var j domain.Job
		err := json.Unmarshal(rec.Body.Bytes(), &j)
		require.NoError(t, err)
		assert.Equal(t, job.ID, j.ID)
		assert.Equal(t, "Backend Developer", j.Title)
	})

	t.Run("GET /v1/public/departments returns active departments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/public/departments", nil)
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string][]string
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, []string{"Engineering", "Product"}, body["departments"])
	})

	t.Run("POST /v1/public/jobs/{job_id}/apply creates application", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		require.NoError(t, writer.WriteField("candidate_name", "Jane Dev"))
		require.NoError(t, writer.WriteField("candidate_email", "jane.dev@example.com"))
		require.NoError(t, writer.WriteField("custom_answers", `{"years_exp": 4}`))

		part, err := writer.CreateFormFile("resume", "jane_resume.pdf")
		require.NoError(t, err)
		_, err = part.Write([]byte("%PDF-1.4 Jane's resume content"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/v1/public/jobs/"+job.ID+"/apply", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-API-Key", pubRaw)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "/v1/admin/applications/")
	})
}

func TestRouterAdminEndpoints(t *testing.T) {
	apiKeyRepo := newMockApiKeyRepo()
	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	strg := storage.NewMemoryStorage()

	adminRaw, adminKey, err := auth.GenerateKey("Admin Key", domain.ApiKeyScopeAdmin)
	require.NoError(t, err)
	adminKey.ID = "admin-key-1"
	require.NoError(t, apiKeyRepo.Create(context.Background(), adminKey))

	pubRaw, pubKey, err := auth.GenerateKey("Public Key", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	pubKey.ID = "pub-key-1"
	require.NoError(t, apiKeyRepo.Create(context.Background(), pubKey))

	router := newTestRouterWithRepos(nil, apiKeyRepo, jobRepo, appRepo, strg, 20, 50)

	var createdJobID string

	t.Run("POST /v1/admin/jobs creates job with admin auth", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"title":                "Staff Go Engineer",
			"department":           "Engineering",
			"location":             "Remote",
			"description_markdown": "Staff role",
			"status":               "draft",
		}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "/v1/admin/jobs/")

		var job domain.Job
		err = json.Unmarshal(rec.Body.Bytes(), &job)
		require.NoError(t, err)
		assert.Equal(t, "Staff Go Engineer", job.Title)
		assert.NotEmpty(t, job.ID)
		createdJobID = job.ID
	})

	t.Run("POST /v1/admin/jobs returns 403 Forbidden with public key", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"title":                "Unauthorized Role",
			"department":           "Engineering",
			"location":             "Remote",
			"description_markdown": "Unauthorized",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GET /v1/admin/jobs returns list of all jobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp api.PaginatedResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.Pagination.TotalItems, 1)
	})

	t.Run("PATCH /v1/admin/jobs/{id} updates job", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"status": "published",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/jobs/"+createdJobID, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var job domain.Job
		err := json.Unmarshal(rec.Body.Bytes(), &job)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusPublished, job.Status)
	})

	var createdAppID string

	t.Run("Create application and test admin candidate management pipeline", func(t *testing.T) {
		app := &domain.Application{
			ID:             "app-admin-test-1",
			JobID:          createdJobID,
			CandidateName:  "Test Candidate",
			CandidateEmail: "candidate@test.com",
			ResumeS3Key:    "resumes/app-admin-test-1/resume.pdf",
			ResumeFilename: "resume.pdf",
			Stage:          domain.ApplicationStageApplied,
			CreatedAt:      time.Now(),
		}
		require.NoError(t, appRepo.Create(context.Background(), app))
		require.NoError(t, strg.Upload(context.Background(), app.ResumeS3Key, strings.NewReader("pdf-content"), 11, "application/pdf"))
		createdAppID = app.ID

		// GET /v1/admin/jobs/{id}/applications
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/"+createdJobID+"/applications", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// GET /v1/admin/applications/{id}
		req = httptest.NewRequest(http.MethodGet, "/v1/admin/applications/"+createdAppID, nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// GET /v1/admin/applications/{id}/resume
		req = httptest.NewRequest(http.MethodGet, "/v1/admin/applications/"+createdAppID+"/resume", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// PATCH /v1/admin/applications/{id}/stage
		stageBody, _ := json.Marshal(map[string]interface{}{"stage": "interviewing"})
		req = httptest.NewRequest(http.MethodPatch, "/v1/admin/applications/"+createdAppID+"/stage", bytes.NewReader(stageBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// POST /v1/admin/applications/{id}/notes
		noteBody, _ := json.Marshal(map[string]interface{}{"author_name": "Reviewer", "note_text": "Strong communication"})
		req = httptest.NewRequest(http.MethodPost, "/v1/admin/applications/"+createdAppID+"/notes", bytes.NewReader(noteBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// GET /v1/admin/applications/{id}/notes
		req = httptest.NewRequest(http.MethodGet, "/v1/admin/applications/"+createdAppID+"/notes", nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("DELETE /v1/admin/jobs/{id} deletes job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/admin/jobs/"+createdJobID, nil)
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestFullEndToEndWorkflow_WithWebhooks(t *testing.T) {
	apiKeyRepo := newMockApiKeyRepo()
	adminRaw, adminKey, err := auth.GenerateKey("Admin Key", domain.ApiKeyScopeAdmin)
	require.NoError(t, err)
	require.NoError(t, apiKeyRepo.Create(context.Background(), adminKey))

	pubRaw, pubKey, err := auth.GenerateKey("Public Key", domain.ApiKeyScopePublic)
	require.NoError(t, err)
	require.NoError(t, apiKeyRepo.Create(context.Background(), pubKey))

	jobRepo := newMockJobRepo()
	appRepo := newMockAppRepo()
	webhookRepo := newMockWebhookRepo()
	strg := storage.NewMemoryStorage()

	// Channel to collect received webhooks in test receiver
	type receivedWebhook struct {
		Event     string
		Payload   domain.WebhookPayload
		Signature string
		Timestamp string
		RawBody   []byte
	}
	webhookCh := make(chan receivedWebhook, 10)

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var payload domain.WebhookPayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		webhookCh <- receivedWebhook{
			Event:     payload.Event,
			Payload:   payload,
			Signature: r.Header.Get(webhook.HeaderSignature),
			Timestamp: r.Header.Get(webhook.HeaderTimestamp),
			RawBody:   bodyBytes,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	dispatcher := webhook.NewDispatcher(webhookRepo, &webhook.DispatcherConfig{
		HTTPTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	webhookSvc := service.NewWebhookService(webhookRepo, dispatcher)
	jobSvc := service.NewJobService(jobRepo, webhookSvc)
	appSvc := service.NewApplicationService(jobRepo, appRepo, newMockNoteRepo(), strg, webhookSvc)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:               8080,
			Env:                "test",
			LogLevel:           "error",
			CORSAllowedOrigins: []string{"*"},
			RateLimitRPS:       20,
			RateLimitBurst:     50,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := api.NewRouter(api.RouterConfig{
		Config:         cfg,
		Logger:         logger,
		ApiKeyRepo:     apiKeyRepo,
		JobService:     jobSvc,
		AppService:     appSvc,
		WebhookService: webhookSvc,
	})

	secretToken := "whsec_custom_secret_12345"

	// 1. Register Webhook Subscription via POST /v1/admin/webhooks
	t.Run("1. Register webhook subscription", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"target_url":   webhookServer.URL,
			"secret_token": secretToken,
			"events":       []string{domain.EventJobPublished, domain.EventApplicationCreated, domain.EventApplicationStageUpdated},
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/webhooks", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "/v1/admin/webhooks/")
	})

	// 2. Create and publish job -> triggers job.published webhook
	var createdJobID string
	t.Run("2. Create published job triggers job.published webhook", func(t *testing.T) {
		jobReq, _ := json.Marshal(map[string]interface{}{
			"title":                "Distributed Systems Engineer",
			"department":           "Infrastructure",
			"location":             "Remote",
			"employment_type":      "full_time",
			"description_markdown": "Work on core systems",
			"status":               "published",
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs", bytes.NewReader(jobReq))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		var job domain.Job
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
		createdJobID = job.ID

		select {
		case wh := <-webhookCh:
			assert.Equal(t, domain.EventJobPublished, wh.Event)
			assert.NotEmpty(t, wh.Signature)
			// Verify signature
			valid := webhook.VerifySignature(secretToken, parseTimestamp(wh.Timestamp), wh.RawBody, wh.Signature, 5*time.Minute)
			assert.True(t, valid, "Webhook HMAC signature must be valid")
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for job.published webhook")
		}
	})

	// 3. Candidate applies via POST /v1/public/jobs/{job_id}/apply -> triggers application.created
	var createdAppID string
	t.Run("3. Candidate applies triggers application.created webhook", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("candidate_name", "Diana Prince")
		_ = writer.WriteField("candidate_email", "diana@example.com")
		part, err := writer.CreateFormFile("resume", "resume.pdf")
		require.NoError(t, err)
		_, _ = part.Write(sampleValidPDF())
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/v1/public/jobs/"+createdJobID+"/apply", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-API-Key", pubRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		var app domain.Application
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &app))
		createdAppID = app.ID

		select {
		case wh := <-webhookCh:
			assert.Equal(t, domain.EventApplicationCreated, wh.Event)
			assert.NotEmpty(t, wh.Signature)
			valid := webhook.VerifySignature(secretToken, parseTimestamp(wh.Timestamp), wh.RawBody, wh.Signature, 5*time.Minute)
			assert.True(t, valid, "Webhook HMAC signature must be valid")
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for application.created webhook")
		}
	})

	// 4. Update stage -> triggers application.stage_updated webhook
	t.Run("4. Transition stage triggers application.stage_updated webhook", func(t *testing.T) {
		stageReq, _ := json.Marshal(map[string]interface{}{
			"stage": "interviewing",
		})
		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/applications/"+createdAppID+"/stage", bytes.NewReader(stageReq))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", adminRaw)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		select {
		case wh := <-webhookCh:
			assert.Equal(t, domain.EventApplicationStageUpdated, wh.Event)
			assert.NotEmpty(t, wh.Signature)
			valid := webhook.VerifySignature(secretToken, parseTimestamp(wh.Timestamp), wh.RawBody, wh.Signature, 5*time.Minute)
			assert.True(t, valid, "Webhook HMAC signature must be valid")
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for application.stage_updated webhook")
		}
	})
}

func parseTimestamp(ts string) int64 {
	var val int64
	for _, c := range ts {
		if c >= '0' && c <= '9' {
			val = val*10 + int64(c-'0')
		}
	}
	return val
}

func sampleValidPDF() []byte {
	return []byte("%PDF-1.4 sample pdf content for unit testing")
}
