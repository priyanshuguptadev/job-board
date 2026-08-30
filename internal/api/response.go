package api

import (
	"net/http"

	"github.com/priyanshuguptadev/job-board/internal/api/httputil"
	"github.com/priyanshuguptadev/job-board/internal/domain"
)

// Standard error codes catalog.
const (
	ErrCodeInvalidAPIKey        = httputil.ErrCodeInvalidAPIKey
	ErrCodeForbidden            = httputil.ErrCodeForbidden
	ErrCodeNotFound             = httputil.ErrCodeNotFound
	ErrCodeMethodNotAllowed     = httputil.ErrCodeMethodNotAllowed
	ErrCodeValidationError      = httputil.ErrCodeValidationError
	ErrCodeUnsupportedMediaType = httputil.ErrCodeUnsupportedMediaType
	ErrCodeRateLimitExceeded    = httputil.ErrCodeRateLimitExceeded
	ErrCodeConflict             = httputil.ErrCodeConflict
	ErrCodeInternalServerError  = httputil.ErrCodeInternalServerError
)

// Re-export types from httputil.
type ErrorDetail = domain.ErrorDetail
type ErrorResponse = httputil.ErrorResponse
type ErrorBody = httputil.ErrorBody
type PaginationMeta = httputil.PaginationMeta
type PaginatedResponse = httputil.PaginatedResponse

// RespondJSON marshals data to JSON and writes it with the provided HTTP status code.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	httputil.RespondJSON(w, status, data)
}

// RespondError sends a standardized error response.
func RespondError(w http.ResponseWriter, status int, code, message string, details ...ErrorDetail) {
	httputil.RespondError(w, status, code, message, details...)
}
