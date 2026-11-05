package domain

import (
	"strings"
	"time"
)

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewTask(title, description string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrInvalidTitle
	}
	now := time.Now().UTC()
	return Task{
		Title: title, Description: strings.TrimSpace(description),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (t *Task) Update(title, description string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrInvalidTitle
	}
	t.Title = title
	t.Description = strings.TrimSpace(description)
	t.UpdatedAt = time.Now().UTC()
	return nil
}
