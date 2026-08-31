package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshuguptadev/job-board/internal/api/httputil"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
)

// AdminHandler handles HTTP requests for administrative ATS and job management endpoints.
type AdminHandler struct {
	jobService     service.JobService
	appService     service.ApplicationService
	webhookService service.WebhookService
	logger         *slog.Logger
}

// NewAdminHandler creates a new AdminHandler instance.
func NewAdminHandler(jobService service.JobService, appService service.ApplicationService, webhookService service.WebhookService, logger *slog.Logger) *AdminHandler {
	if jobService == nil {
		panic("v1: jobService is required")
	}
	if appService == nil {
		panic("v1: appService is required")
	}
	return &AdminHandler{
		jobService:     jobService,
		appService:     appService,
		webhookService: webhookService,
		logger:         logger,
	}
}

// CreateWebhookRequest represents the payload for registering an outbound webhook.
type CreateWebhookRequest struct {
	TargetURL   string   `json:"target_url"`
	SecretToken *string  `json:"secret_token,omitempty"`
	Events      []string `json:"events"`
}

// CreateJobRequest represents the payload for creating a new job listing.
type CreateJobRequest struct {
	Slug                string                `json:"slug,omitempty"`
	Title               string                `json:"title"`
	Department          string                `json:"department"`
	Location            string                `json:"location"`
	EmploymentType      domain.EmploymentType `json:"employment_type,omitempty"`
	ExperienceLevel     *string               `json:"experience_level,omitempty"`
	SalaryMin           *float64              `json:"salary_min,omitempty"`
	SalaryMax           *float64              `json:"salary_max,omitempty"`
	SalaryCurrency      string                `json:"salary_currency,omitempty"`
	DescriptionMarkdown string                `json:"description_markdown"`
	Status              domain.JobStatus      `json:"status,omitempty"`
	CustomFields        []domain.CustomField  `json:"custom_fields,omitempty"`
}

// UpdateJobRequest represents the payload for updating an existing job listing.
type UpdateJobRequest struct {
	Slug                *string                `json:"slug,omitempty"`
	Title               *string                `json:"title,omitempty"`
	Department          *string                `json:"department,omitempty"`
	Location            *string                `json:"location,omitempty"`
	EmploymentType      *domain.EmploymentType `json:"employment_type,omitempty"`
	ExperienceLevel     *string                `json:"experience_level,omitempty"`
	SalaryMin           *float64               `json:"salary_min,omitempty"`
	SalaryMax           *float64               `json:"salary_max,omitempty"`
	SalaryCurrency      *string                `json:"salary_currency,omitempty"`
	DescriptionMarkdown *string                `json:"description_markdown,omitempty"`
	Status              *domain.JobStatus      `json:"status,omitempty"`
	CustomFields        *[]domain.CustomField  `json:"custom_fields,omitempty"`
}

// UpdateStageRequest represents the payload for updating candidate hiring stage.
type UpdateStageRequest struct {
	Stage          domain.ApplicationStage `json:"stage"`
	RejectedReason *string                 `json:"rejected_reason,omitempty"`
}

// CreateNoteRequest represents the payload for creating an internal review note.
type CreateNoteRequest struct {
	AuthorName string `json:"author_name"`
	NoteText   string `json:"note_text"`
}

// CreateJob handles POST /v1/admin/jobs.
func (h *AdminHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "body",
			Issue: "Invalid JSON request body",
		})
		return
	}

	input := service.CreateJobInput{
		Slug:                req.Slug,
		Title:               req.Title,
		Department:          req.Department,
		Location:            req.Location,
		EmploymentType:      req.EmploymentType,
		ExperienceLevel:     req.ExperienceLevel,
		SalaryMin:           req.SalaryMin,
		SalaryMax:           req.SalaryMax,
		SalaryCurrency:      req.SalaryCurrency,
		DescriptionMarkdown: req.DescriptionMarkdown,
		Status:              req.Status,
		CustomFields:        req.CustomFields,
	}

	job, err := h.jobService.CreateJob(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			httputil.RespondError(w, http.StatusConflict, httputil.ErrCodeConflict, "A job with this slug already exists.")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to create job", "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to create job.")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/admin/jobs/%s", job.ID))
	httputil.RespondJSON(w, http.StatusCreated, job)
}

// ListJobs handles GET /v1/admin/jobs.
func (h *AdminHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	filter, details := parseAdminJobListQuery(r)
	if len(details) > 0 {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", details...)
		return
	}

	jobs, totalItems, err := h.jobService.ListJobs(r.Context(), filter)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("failed to list jobs", "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve job listings.")
		return
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + filter.Limit - 1) / filter.Limit
	}

	httputil.RespondJSON(w, http.StatusOK, httputil.PaginatedResponse{
		Data: jobs,
		Pagination: httputil.PaginationMeta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	})
}

// GetJob handles GET /v1/admin/jobs/{id}.
func (h *AdminHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
		return
	}

	job, err := h.jobService.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to retrieve job", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve job details.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, job)
}

// UpdateJob handles PATCH /v1/admin/jobs/{id}.
func (h *AdminHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
		return
	}

	var req UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "body",
			Issue: "Invalid JSON request body",
		})
		return
	}

	input := service.UpdateJobInput{
		Slug:                req.Slug,
		Title:               req.Title,
		Department:          req.Department,
		Location:            req.Location,
		EmploymentType:      req.EmploymentType,
		ExperienceLevel:     req.ExperienceLevel,
		SalaryMin:           req.SalaryMin,
		SalaryMax:           req.SalaryMax,
		SalaryCurrency:      req.SalaryCurrency,
		DescriptionMarkdown: req.DescriptionMarkdown,
		Status:              req.Status,
		CustomFields:        req.CustomFields,
	}

	job, err := h.jobService.UpdateJob(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			httputil.RespondError(w, http.StatusConflict, httputil.ErrCodeConflict, "A job with this slug already exists.")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to update job", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to update job.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, job)
}

// DeleteJob handles DELETE /v1/admin/jobs/{id}.
func (h *AdminHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
		return
	}

	err := h.jobService.DeleteJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to delete job", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to delete job.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListJobApplications handles GET /v1/admin/jobs/{id}/applications.
func (h *AdminHandler) ListJobApplications(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
		return
	}

	filter, details := parseApplicationListQuery(r)
	if len(details) > 0 {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", details...)
		return
	}
	filter.JobID = jobID

	apps, totalItems, err := h.appService.ListApplications(r.Context(), filter)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to list applications", "job_id", jobID, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve applications.")
		return
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + filter.Limit - 1) / filter.Limit
	}

	httputil.RespondJSON(w, http.StatusOK, httputil.PaginatedResponse{
		Data: apps,
		Pagination: httputil.PaginationMeta{
			Page:       filter.Page,
			Limit:      filter.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	})
}

// GetApplication handles GET /v1/admin/applications/{id}.
func (h *AdminHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
		return
	}

	app, err := h.appService.GetApplication(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to retrieve application", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve application details.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, app)
}

// GetResumeURL handles GET /v1/admin/applications/{id}/resume.
func (h *AdminHandler) GetResumeURL(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
		return
	}

	res, err := h.appService.GetResumeDownloadURL(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to generate resume presigned url", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to generate resume download link.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, res)
}

// UpdateStage handles PATCH /v1/admin/applications/{id}/stage.
func (h *AdminHandler) UpdateStage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
		return
	}

	var req UpdateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "body",
			Issue: "Invalid JSON request body",
		})
		return
	}

	input := service.UpdateStageInput{
		Stage:          req.Stage,
		RejectedReason: req.RejectedReason,
	}

	app, err := h.appService.UpdateStage(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to update application stage", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to update application stage.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, app)
}

// CreateNote handles POST /v1/admin/applications/{id}/notes.
func (h *AdminHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
		return
	}

	var req CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "body",
			Issue: "Invalid JSON request body",
		})
		return
	}

	input := service.CreateNoteInput{
		AuthorName: req.AuthorName,
		NoteText:   req.NoteText,
	}

	note, err := h.appService.CreateNote(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to create application note", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to create application note.")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/admin/applications/%s/notes/%s", id, note.ID))
	httputil.RespondJSON(w, http.StatusCreated, note)
}

// ListNotes handles GET /v1/admin/applications/{id}/notes.
func (h *AdminHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
		return
	}

	notes, err := h.appService.ListNotes(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested application was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to list application notes", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve application notes.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, notes)
}

func parseAdminJobListQuery(r *http.Request) (domain.JobListFilter, []domain.ErrorDetail) {
	query := r.URL.Query()
	var details []domain.ErrorDetail

	statusStr := strings.TrimSpace(query.Get("status"))
	var status domain.JobStatus
	if statusStr != "" {
		switch domain.JobStatus(statusStr) {
		case domain.JobStatusDraft, domain.JobStatusPublished, domain.JobStatusArchived:
			status = domain.JobStatus(statusStr)
		default:
			details = append(details, domain.ErrorDetail{
				Field: "status",
				Issue: "Invalid status. Must be one of: draft, published, archived",
			})
		}
	}

	empTypeStr := strings.TrimSpace(query.Get("employment_type"))
	var empType domain.EmploymentType
	if empTypeStr != "" {
		switch domain.EmploymentType(empTypeStr) {
		case domain.EmploymentTypeFullTime, domain.EmploymentTypePartTime, domain.EmploymentTypeContract, domain.EmploymentTypeInternship:
			empType = domain.EmploymentType(empTypeStr)
		default:
			details = append(details, domain.ErrorDetail{
				Field: "employment_type",
				Issue: "Invalid employment_type. Must be one of: full_time, part_time, contract, internship",
			})
		}
	}

	page := 1
	if pStr := strings.TrimSpace(query.Get("page")); pStr != "" {
		p, err := strconv.Atoi(pStr)
		if err != nil || p < 1 {
			details = append(details, domain.ErrorDetail{
				Field: "page",
				Issue: "page must be a positive integer",
			})
		} else {
			page = p
		}
	}

	limit := 20
	if lStr := strings.TrimSpace(query.Get("limit")); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 {
			details = append(details, domain.ErrorDetail{
				Field: "limit",
				Issue: "limit must be a positive integer",
			})
		} else if l > 100 {
			limit = 100
		} else {
			limit = l
		}
	}

	filter := domain.JobListFilter{
		Status:         status,
		Department:     strings.TrimSpace(query.Get("department")),
		Location:       strings.TrimSpace(query.Get("location")),
		EmploymentType: empType,
		Page:           page,
		Limit:          limit,
	}

	return filter, details
}

func parseApplicationListQuery(r *http.Request) (domain.ApplicationListFilter, []domain.ErrorDetail) {
	query := r.URL.Query()
	var details []domain.ErrorDetail

	stageStr := strings.TrimSpace(query.Get("stage"))
	var stage domain.ApplicationStage
	if stageStr != "" {
		if !domain.IsValidStage(domain.ApplicationStage(stageStr)) {
			details = append(details, domain.ErrorDetail{
				Field: "stage",
				Issue: "Invalid stage. Must be one of: applied, screening, interviewing, offer, hired, rejected",
			})
		} else {
			stage = domain.ApplicationStage(stageStr)
		}
	}

	page := 1
	if pStr := strings.TrimSpace(query.Get("page")); pStr != "" {
		p, err := strconv.Atoi(pStr)
		if err != nil || p < 1 {
			details = append(details, domain.ErrorDetail{
				Field: "page",
				Issue: "page must be a positive integer",
			})
		} else {
			page = p
		}
	}

	limit := 20
	if lStr := strings.TrimSpace(query.Get("limit")); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 {
			details = append(details, domain.ErrorDetail{
				Field: "limit",
				Issue: "limit must be a positive integer",
			})
		} else if l > 100 {
			limit = 100
		} else {
			limit = l
		}
	}

	filter := domain.ApplicationListFilter{
		Stage:          stage,
		CandidateEmail: strings.TrimSpace(query.Get("candidate_email")),
		Page:           page,
		Limit:          limit,
	}

	return filter, details
}

// CreateWebhook handles POST /v1/admin/webhooks.
func (h *AdminHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookService == nil {
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Webhook service not available.")
		return
	}

	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "body",
			Issue: "Invalid JSON request body",
		})
		return
	}

	input := service.CreateWebhookSubscriptionInput{
		TargetURL:   req.TargetURL,
		SecretToken: req.SecretToken,
		Events:      req.Events,
	}

	sub, err := h.webhookService.CreateSubscription(r.Context(), input)
	if err != nil {
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to create webhook subscription", "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to create webhook subscription.")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/admin/webhooks/%s", sub.ID))
	httputil.RespondJSON(w, http.StatusCreated, sub)
}

// ListWebhooks handles GET /v1/admin/webhooks.
func (h *AdminHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	if h.webhookService == nil {
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Webhook service not available.")
		return
	}

	subs, err := h.webhookService.ListSubscriptions(r.Context())
	if err != nil {
		if h.logger != nil {
			h.logger.Error("failed to list webhook subscriptions", "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve webhook subscriptions.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, subs)
}

// DeleteWebhook handles DELETE /v1/admin/webhooks/{id}.
func (h *AdminHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookService == nil {
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Webhook service not available.")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested webhook subscription was not found.")
		return
	}

	err := h.webhookService.DeleteSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested webhook subscription was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to delete webhook subscription", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to delete webhook subscription.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestWebhook handles POST /v1/admin/webhooks/{id}/test.
func (h *AdminHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookService == nil {
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Webhook service not available.")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested webhook subscription was not found.")
		return
	}

	res, err := h.webhookService.TestSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested webhook subscription was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to test webhook subscription", "id", id, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to execute webhook test ping.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, res)
}
