package domain

import "errors"

// Common domain errors.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists or conflict detected")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInternal     = errors.New("internal error")
)
