package domain

import "testing"

func TestNewTaskRejectsBlankTitle(t *testing.T) {
	_, err := NewTask("   ", "description")
	if err != ErrInvalidTitle {
		t.Fatalf("error = %v, want %v", err, ErrInvalidTitle)
	}
}
