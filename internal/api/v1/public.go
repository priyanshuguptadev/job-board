package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshuguptadev/job-board/internal/api/httputil"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/priyanshuguptadev/job-board/internal/service"
	"github.com/priyanshuguptadev/job-board/internal/storage"
)

const (
	maxFormMemoryBuffer = 12 * 1024 * 1024 // 12MB memory buffer for multipart form parsing
	maxRequestBodySize  = 15 * 1024 * 1024 // 15MB total HTTP request body limit
)

// PublicHandler handles HTTP requests for public career endpoints.
type PublicHandler struct {
	jobService service.JobService
	appService service.ApplicationService
	logger     *slog.Logger
}

// NewPublicHandler creates a new PublicHandler.
func NewPublicHandler(jobService service.JobService, appService service.ApplicationService, logger *slog.Logger) *PublicHandler {
	if jobService == nil {
		panic("v1: jobService is required")
	}
	if appService == nil {
		panic("v1: appService is required")
	}
	return &PublicHandler{
		jobService: jobService,
		appService: appService,
		logger:     logger,
	}
}

// ApplicationSubmissionResponse represents the response body returned upon successful application submission.
type ApplicationSubmissionResponse struct {
	ID             string                  `json:"id"`
	JobID          string                  `json:"job_id"`
	CandidateName  string                  `json:"candidate_name"`
	CandidateEmail string                  `json:"candidate_email"`
	CandidatePhone *string                 `json:"candidate_phone,omitempty"`
	LinkedInURL    *string                 `json:"linkedin_url,omitempty"`
	ResumeFilename string                  `json:"resume_filename"`
	CustomAnswers  map[string]interface{}  `json:"custom_answers"`
	Stage          domain.ApplicationStage `json:"stage"`
	CreatedAt      time.Time               `json:"created_at"`
	Message        string                  `json:"message"`
}

// ListJobs handles GET /v1/public/jobs.
func (h *PublicHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	filter, details := parseJobListQuery(r)
	if len(details) > 0 {
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", details...)
		return
	}

	jobs, totalItems, err := h.jobService.ListPublishedJobs(r.Context(), filter)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("failed to list published jobs", "error", err)
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

// GetJob handles GET /v1/public/jobs/{slug_or_id}.
func (h *PublicHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	slugOrID := strings.TrimSpace(chi.URLParam(r, "slug_or_id"))
	if slugOrID == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job posting was not found.")
		return
	}

	job, err := h.jobService.GetPublishedJob(r.Context(), slugOrID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job posting was not found.")
			return
		}
		if h.logger != nil {
			h.logger.Error("failed to retrieve job", "slug_or_id", slugOrID, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve job details.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, job)
}

// ListDepartments handles GET /v1/public/departments.
func (h *PublicHandler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	departments, err := h.jobService.ListDepartments(r.Context())
	if err != nil {
		if h.logger != nil {
			h.logger.Error("failed to list departments", "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to retrieve departments.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"departments": departments,
	})
}

// Apply handles POST /v1/public/jobs/{job_id}/apply.
func (h *PublicHandler) Apply(w http.ResponseWriter, r *http.Request) {
	jobIDOrSlug := strings.TrimSpace(chi.URLParam(r, "job_id"))
	if jobIDOrSlug == "" {
		httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found or is not active.")
		return
	}

	// Limit request body to prevent unbounded memory allocation
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxFormMemoryBuffer); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
				Field: "resume",
				Issue: "File exceeds the maximum allowed size of 10MB",
			})
			return
		}
		httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
			Field: "Content-Type",
			Issue: "Invalid multipart/form-data request body",
		})
		return
	}

	candidateName := strings.TrimSpace(r.FormValue("candidate_name"))
	candidateEmail := strings.TrimSpace(r.FormValue("candidate_email"))

	var candidatePhone *string
	if p := strings.TrimSpace(r.FormValue("candidate_phone")); p != "" {
		candidatePhone = &p
	}

	var linkedinURL *string
	if u := strings.TrimSpace(r.FormValue("linkedin_url")); u != "" {
		linkedinURL = &u
	}

	var customAnswers map[string]interface{}
	customAnswersRaw := strings.TrimSpace(r.FormValue("custom_answers"))
	if customAnswersRaw != "" {
		if err := json.Unmarshal([]byte(customAnswersRaw), &customAnswers); err != nil {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", domain.ErrorDetail{
				Field: "custom_answers",
				Issue: "custom_answers must be a valid JSON object",
			})
			return
		}
	}

	var resumeReader io.Reader
	var resumeFilename string
	var resumeSize int64
	var resumeContentType string

	file, fileHeader, err := r.FormFile("resume")
	if err == nil && file != nil {
		defer file.Close()
		resumeReader = file
		resumeFilename = fileHeader.Filename
		resumeSize = fileHeader.Size
		resumeContentType = fileHeader.Header.Get("Content-Type")
	}

	input := service.ApplyInput{
		JobIDOrSlug:             jobIDOrSlug,
		CandidateName:           candidateName,
		CandidateEmail:          candidateEmail,
		CandidatePhone:          candidatePhone,
		LinkedInURL:             linkedinURL,
		ResumeFilename:          resumeFilename,
		ResumeSize:              resumeSize,
		ResumeReader:            resumeReader,
		ResumeHeaderContentType: resumeContentType,
		CustomAnswers:           customAnswers,
	}

	app, err := h.appService.Apply(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "The requested job was not found or is not active.")
			return
		}
		if errors.Is(err, domain.ErrUnsupportedMediaType) || errors.Is(err, storage.ErrUnsupportedMediaType) {
			httputil.RespondError(w, http.StatusUnsupportedMediaType, httputil.ErrCodeUnsupportedMediaType, "Resume upload is not an allowed MIME type (PDF, DOC, DOCX).")
			return
		}
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			httputil.RespondError(w, http.StatusUnprocessableEntity, httputil.ErrCodeValidationError, "The provided input failed validation checks.", valErr.Details...)
			return
		}

		if h.logger != nil {
			h.logger.Error("failed to submit application", "job_id", jobIDOrSlug, "error", err)
		}
		httputil.RespondError(w, http.StatusInternalServerError, httputil.ErrCodeInternalServerError, "Failed to submit application.")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/admin/applications/%s", app.ID))
	httputil.RespondJSON(w, http.StatusCreated, ApplicationSubmissionResponse{
		ID:             app.ID,
		JobID:          app.JobID,
		CandidateName:  app.CandidateName,
		CandidateEmail: app.CandidateEmail,
		CandidatePhone: app.CandidatePhone,
		LinkedInURL:    app.LinkedInURL,
		ResumeFilename: app.ResumeFilename,
		CustomAnswers:  app.CustomAnswers,
		Stage:          app.Stage,
		CreatedAt:      app.CreatedAt,
		Message:        "Application submitted successfully",
	})
}

func parseJobListQuery(r *http.Request) (domain.JobListFilter, []domain.ErrorDetail) {
	query := r.URL.Query()
	var details []domain.ErrorDetail

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
		Department:     strings.TrimSpace(query.Get("department")),
		Location:       strings.TrimSpace(query.Get("location")),
		EmploymentType: empType,
		Page:           page,
		Limit:          limit,
	}

	return filter, details
}
