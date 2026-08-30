package httputil

import (
	"encoding/json"
	"net/http"

	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// Standard error codes catalog.
const (
	ErrCodeInvalidAPIKey        = "INVALID_API_KEY"
	ErrCodeForbidden            = "FORBIDDEN"
	ErrCodeNotFound             = "NOT_FOUND"
	ErrCodeMethodNotAllowed     = "METHOD_NOT_ALLOWED"
	ErrCodeValidationError      = "VALIDATION_ERROR"
	ErrCodeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"
	ErrCodeRateLimitExceeded    = "RATE_LIMIT_EXCEEDED"
	ErrCodeConflict             = "CONFLICT"
	ErrCodeInternalServerError  = "INTERNAL_SERVER_ERROR"
)

// ErrorDetail describes an issue with a specific input field.
type ErrorDetail = domain.ErrorDetail

// ErrorResponse represents the unified error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains error code, human-readable message, and optional details.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// PaginationMeta provides pagination information for collection queries.
type PaginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// PaginatedResponse wraps a collection of items with pagination metadata.
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// RespondJSON marshals data to JSON and writes it with the provided HTTP status code.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondError sends a standardized error response.
func RespondError(w http.ResponseWriter, status int, code, message string, details ...ErrorDetail) {
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	RespondJSON(w, status, resp)
}
