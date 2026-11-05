package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/go-crud/go-task-api/internal/domain"
)

func TestConcurrentCreateProducesUniqueIDs(t *testing.T) {
	repo := New()
	const workers = 100
	ids := make(chan int64, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := repo.Create(context.Background(), domain.Task{Title: "concurrent"})
			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}
			ids <- task.ID
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]struct{}, workers)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID %d", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers {
		t.Fatalf("got %d IDs, want %d", len(seen), workers)
	}
}

func TestDeleteDoesNotReuseID(t *testing.T) {
	repo := New()
	ctx := context.Background()
	first, _ := repo.Create(ctx, domain.Task{Title: "first"})
	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := repo.Create(ctx, domain.Task{Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID+1 {
		t.Fatalf("ID = %d, want %d", second.ID, first.ID+1)
	}
}
