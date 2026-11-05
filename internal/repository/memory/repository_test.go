package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-crud/go-task-api/internal/domain"
)

func TestCRUDLifecycle(t *testing.T) {
	repo, ctx := New(), context.Background()

	created, err := repo.Create(ctx, domain.Task{Title: "a", Description: "d"})
	if err != nil || created.ID != 1 {
		t.Fatalf("Create = %+v, %v", created, err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil || got.Title != "a" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}

	created.Title, created.Completed = "b", true
	updated, err := repo.Update(ctx, created)
	if err != nil || updated.Title != "b" || !updated.Completed {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	got, _ = repo.GetByID(ctx, created.ID)
	if got.Title != "b" {
		t.Fatalf("update not persisted: %+v", got)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete error = %v, want ErrNotFound", err)
	}
}

func TestNotFound(t *testing.T) {
	repo, ctx := New(), context.Background()
	if _, err := repo.GetByID(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID error = %v", err)
	}
	if _, err := repo.Update(ctx, domain.Task{ID: 99, Title: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update error = %v", err)
	}
	if err := repo.Delete(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := repo.Update(ctx, domain.Task{ID: 99}); repo.tasks[99].ID != 0 || err == nil {
		t.Fatal("Update of a missing task must not create it")
	}
}

func TestListEmptyAndSorted(t *testing.T) {
	repo, ctx := New(), context.Background()
	got, err := repo.List(ctx)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty List = %#v, %v; want non-nil empty slice (JSON [] not null)", got, err)
	}
	for _, title := range []string{"a", "b", "c", "d"} {
		_, _ = repo.Create(ctx, domain.Task{Title: title})
	}
	_ = repo.Delete(ctx, 2)
	got, _ = repo.List(ctx)
	ids := []int64{}
	for _, task := range got {
		ids = append(ids, task.ID)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 3 || ids[2] != 4 {
		t.Fatalf("ids = %v, want [1 3 4]", ids)
	}
}

func TestListReturnsCopy(t *testing.T) {
	repo, ctx := New(), context.Background()
	_, _ = repo.Create(ctx, domain.Task{Title: "orig"})
	list, _ := repo.List(ctx)
	list[0].Title = "mutated"
	again, _ := repo.GetByID(ctx, 1)
	if again.Title != "orig" {
		t.Fatal("mutating a returned slice must not affect stored data")
	}
}

func TestCanceledContext(t *testing.T) {
	repo := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Create(ctx, domain.Task{Title: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create error = %v", err)
	}
	if _, err := repo.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v", err)
	}
	if _, err := repo.GetByID(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetByID error = %v", err)
	}
	if _, err := repo.Update(ctx, domain.Task{ID: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Update error = %v", err)
	}
	if err := repo.Delete(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete error = %v", err)
	}
}

// Mixed readers and writers; meaningful under `go test -race`.
func TestConcurrentReadsAndWrites(t *testing.T) {
	repo, ctx := New(), context.Background()
	seed, _ := repo.Create(ctx, domain.Task{Title: "seed"})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(4)
		go func() { defer wg.Done(); _, _ = repo.Create(ctx, domain.Task{Title: "c"}) }()
		go func() { defer wg.Done(); _, _ = repo.List(ctx) }()
		go func() { defer wg.Done(); _, _ = repo.GetByID(ctx, seed.ID) }()
		go func() {
			defer wg.Done()
			s := seed
			s.Title = "updated"
			_, _ = repo.Update(ctx, s)
		}()
	}
	wg.Wait()

	list, _ := repo.List(ctx)
	if len(list) != 21 {
		t.Fatalf("len = %d, want 21", len(list))
	}
}

func TestConcurrentDeleteExactlyOneWins(t *testing.T) {
	repo, ctx := New(), context.Background()
	task, _ := repo.Create(ctx, domain.Task{Title: "x"})

	const n = 50
	results := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.Delete(ctx, task.ID) }()
	}
	wg.Wait()
	close(results)

	ok, notFound := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrNotFound):
			notFound++
		default:
			t.Fatalf("unexpected error %v", err)
		}
	}
	if ok != 1 || notFound != n-1 {
		t.Fatalf("ok=%d notFound=%d, want 1 and %d", ok, notFound, n-1)
	}
}

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewFromJSON(t *testing.T) {
	t.Run("assigns local IDs and skips empty titles", func(t *testing.T) {
		path := writeFixture(t, `[{"id":500,"title":"one"},{"title":""},{"title":"two","completed":true}]`)
		repo, err := NewFromJSON(path)
		if err != nil {
			t.Fatal(err)
		}
		list, _ := repo.List(context.Background())
		if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 || !list[1].Completed {
			t.Fatalf("list = %+v", list)
		}
		next, _ := repo.Create(context.Background(), domain.Task{Title: "three"})
		if next.ID != 3 {
			t.Fatalf("next ID = %d, want 3", next.ID)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if _, err := NewFromJSON(filepath.Join(t.TempDir(), "nope.json")); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("malformed JSON", func(t *testing.T) {
		if _, err := NewFromJSON(writeFixture(t, `{not json`)); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("shipped fixture loads", func(t *testing.T) {
		repo, err := NewFromJSON(filepath.Join("..", "..", "..", "testdata", "tasks.json"))
		if err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(context.Background()); len(list) == 0 {
			t.Fatal("fixture produced no tasks")
		}
	})
}
