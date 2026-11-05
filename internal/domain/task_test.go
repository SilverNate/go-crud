package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		desc      string
		wantTitle string
		wantDesc  string
		wantErr   error
	}{
		{"valid", "Learn Go", "Study", "Learn Go", "Study", nil},
		{"trims whitespace", "  Learn Go  ", "  Study  ", "Learn Go", "Study", nil},
		{"empty description allowed", "Task", "", "Task", "", nil},
		{"empty title", "", "d", "", "", ErrInvalidTitle},
		{"blank title", "   \t\n", "d", "", "", ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().UTC().Add(-time.Second)
			got, err := NewTask(tt.title, tt.desc)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != (Task{}) {
					t.Fatalf("task = %+v, want zero value on error", got)
				}
				return
			}
			if got.Title != tt.wantTitle || got.Description != tt.wantDesc {
				t.Fatalf("got title=%q desc=%q", got.Title, got.Description)
			}
			if got.ID != 0 {
				t.Fatalf("ID = %d, want 0 (assigned by repository)", got.ID)
			}
			if got.Completed {
				t.Fatal("new task must not be completed")
			}
			if got.CreatedAt.Before(before) || !got.CreatedAt.Equal(got.UpdatedAt) {
				t.Fatalf("timestamps not initialised consistently: %v / %v", got.CreatedAt, got.UpdatedAt)
			}
			if got.CreatedAt.Location() != time.UTC {
				t.Fatal("timestamps must be UTC")
			}
		})
	}
}

func TestTaskUpdate(t *testing.T) {
	t.Run("changes fields and bumps UpdatedAt only", func(t *testing.T) {
		orig, _ := NewTask("old", "old desc")
		orig.ID = 7
		orig.Completed = true
		orig.UpdatedAt = orig.UpdatedAt.Add(-time.Hour)
		prevUpdated := orig.UpdatedAt

		if err := orig.Update("  new  ", " new desc "); err != nil {
			t.Fatal(err)
		}
		if orig.Title != "new" || orig.Description != "new desc" {
			t.Fatalf("got %q / %q", orig.Title, orig.Description)
		}
		if !orig.UpdatedAt.After(prevUpdated) {
			t.Fatal("UpdatedAt was not advanced")
		}
		if orig.ID != 7 || !orig.Completed {
			t.Fatal("Update must not touch ID or Completed")
		}
	})

	t.Run("invalid title leaves task unchanged", func(t *testing.T) {
		task, _ := NewTask("keep", "keep desc")
		snapshot := task
		if err := task.Update("   ", "changed"); !errors.Is(err, ErrInvalidTitle) {
			t.Fatalf("error = %v, want ErrInvalidTitle", err)
		}
		if task != snapshot {
			t.Fatalf("task mutated on failed update: %+v", task)
		}
	})
}
