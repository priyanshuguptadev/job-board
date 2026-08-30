package domain

import "errors"

// Common domain errors.
var (
	ErrNotFound             = errors.New("resource not found")
	ErrConflict             = errors.New("resource already exists or conflict detected")
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrInternal             = errors.New("internal error")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
)

// ErrorDetail describes an issue with a specific input field.
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Issue string `json:"issue"`
}

// ValidationError represents an error when input validation fails.
type ValidationError struct {
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "validation error"
}
