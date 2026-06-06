package domain

import "errors"

var (
	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAccessDenied is returned when the caller does not own the resource.
	ErrAccessDenied = errors.New("access denied")
)
