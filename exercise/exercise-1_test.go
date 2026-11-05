package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestTaskStore(t *testing.T) {
	t.Run("Create valid task", func(t *testing.T) {
		store := NewTaskStore()
		task, err := store.CreateTask("First Task", "Description")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.ID != 1 {
			t.Errorf("expected ID 1, got %d", task.ID)
		}
	})

	t.Run("Create second task", func(t *testing.T) {
		store := NewTaskStore()
		_, _ = store.CreateTask("First Task", "Description")
		task2, err := store.CreateTask("Second Task", "Description")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task2.ID != 2 {
			t.Errorf("expected ID 2, got %d", task2.ID)
		}
	})

	t.Run("Empty title", func(t *testing.T) {
		store := NewTaskStore()
		_, err := store.CreateTask("", "Description")
		if err == nil {
			t.Error("expected error for empty title, got nil")
		}
	})

	t.Run("Spaces-only title", func(t *testing.T) {
		store := NewTaskStore()
		_, err := store.CreateTask("   ", "Description")
		if err == nil {
			t.Error("expected error for spaces-only title, got nil")
		}
	})

	t.Run("Get existing ID", func(t *testing.T) {
		store := NewTaskStore()
		created, _ := store.CreateTask("Valid Task", "Description")
		retrieved, err := store.GetTaskByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if retrieved.ID != created.ID {
			t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
		}
	})

	t.Run("Get missing ID", func(t *testing.T) {
		store := NewTaskStore()
		_, err := store.GetTaskByID(999)
		if err == nil {
			t.Error("expected error for missing ID, got nil")
		}
	})

	t.Run("Get ID -1", func(t *testing.T) {
		store := NewTaskStore()
		_, err := store.GetTaskByID(-1)
		if err == nil {
			t.Error("expected error for negative ID, got nil")
		}
	})

	t.Run("Get ID 0", func(t *testing.T) {
		store := NewTaskStore()
		_, err := store.GetTaskByID(0)
		if err == nil {
			t.Error("expected error for negative ID, got nil")
		}
	})

}

// 1. Proves failed creation does not consume an ID
func TestFailedCreationDoesNotConsumeID(t *testing.T) {
	store := NewTaskStore()

	// Attempt failed creations
	_, _ = store.CreateTask("", "Empty title")
	_, _ = store.CreateTask("   ", "Spaces only")

	// Create valid task - should still get ID 1
	validTask, err := store.CreateTask("Valid Task", "Valid Description")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if validTask.ID != 1 {
		t.Errorf("expected ID 1 after failed creations, got %d", validTask.ID)
	}
}

// 2. Verifies retrieved task's title and description
func TestGetTaskByID_FieldAssertions(t *testing.T) {
	store := NewTaskStore()
	expectedTitle := "Go Concurrency"
	expectedDesc := "Learn RWMutex and channels"

	created, err := store.CreateTask(expectedTitle, expectedDesc)
	if err != nil {
		t.Fatalf("expected no error on creation, got %v", err)
	}

	retrieved, err := store.GetTaskByID(created.ID)
	if err != nil {
		t.Fatalf("expected no error on retrieval, got %v", err)
	}

	if retrieved.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, retrieved.Title)
	}

	if retrieved.Description != expectedDesc {
		t.Errorf("expected description %q, got %q", expectedDesc, retrieved.Description)
	}
}

// 3. Channel-based concurrent creation test verifying unique IDs 1 to 100
func TestConcurrentCreateTask(t *testing.T) {
	store := NewTaskStore()
	workers := 100

	// 1. Create a buffered channel with capacity equal to workers
	idChan := make(chan int64, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			task, err := store.CreateTask(fmt.Sprintf("Task %d", n), "Concurrent test")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			// 2. Send task.ID into buffered channel
			idChan <- task.ID
		}(i)
	}

	// 3. Wait for all goroutines and close channel
	wg.Wait()
	close(idChan)

	// 4. Read IDs and track in a separate map
	seenIDs := make(map[int64]bool)
	for id := range idChan {
		// 5. Fail if an ID already exists
		if seenIDs[id] {
			t.Fatalf("duplicate ID detected: %d", id)
		}
		seenIDs[id] = true
	}

	// 6. Verify IDs cover the expected range from 1 to 100
	if len(seenIDs) != workers {
		t.Fatalf("expected %d unique IDs, got %d", workers, len(seenIDs))
	}

	for expectedID := int64(1); expectedID <= int64(workers); expectedID++ {
		if !seenIDs[expectedID] {
			t.Errorf("missing expected ID in range 1..100: %d", expectedID)
		}
	}
}
