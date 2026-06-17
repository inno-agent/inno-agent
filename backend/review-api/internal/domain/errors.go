package domain

import "errors"

var (
	ErrValidation      = errors.New("validation error")
	ErrDiffUnavailable = errors.New("diff unavailable")
)
