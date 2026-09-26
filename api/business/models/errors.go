package models

import "errors"

// Domain errors. handlers/errors.go maps each one to an HTTP status.
var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrCaseClosed   = errors.New("case is closed")
	ErrConflict     = errors.New("case was changed by another request, try again")
)
