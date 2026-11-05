package domain

import "errors"

var (
	ErrInvalidID    = errors.New("task ID must be greater than zero")
	ErrInvalidTitle = errors.New("task title cannot be empty")
	ErrNotFound     = errors.New("task not found")
)
