package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-crud/go-task-api/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- Tests that need NO database -------------------------------------------

func TestNewRequiresDatabaseURL(t *testing.T) {
	if _, err := New(context.Background(), ""); err == nil {
		t.Fatal("want error for empty DATABASE_URL")
	}
}

func TestNewFailsFastWhenDatabaseUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Port 1 is never a PostgreSQL server; New must Ping and report the failure
	// instead of returning a repository that fails on first use.
	if _, err := New(ctx, "postgres://u:p@127.0.0.1:1/x?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("want error for unreachable database")
	}
}

// ---- Integration tests (need PostgreSQL) -----------------------------------
//
// Set TEST_DATABASE_URL to a DISPOSABLE database whose name contains "test",
// e.g. postgres://postgres:postgres@localhost:5432/taskdb_test?sslmode=disable
// Each test run creates its own schema and drops it afterwards, so existing
// data is never touched. Without the variable these tests are skipped.

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if name := strings.ToLower(strings.TrimPrefix(u.Path, "/")); !strings.Contains(name, "test") {
		t.Fatalf("refusing to run: database name %q must contain \"test\"", name)
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("it_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	repo, err := New(ctx, u.String())
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		repo.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "001_create_tasks.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return repo
}

func newTask(title string) domain.Task {
	task, err := domain.NewTask(title, "desc")
	if err != nil {
		panic(err)
	}
	return task
}

func TestCRUDRoundTrip(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()

	created, err := repo.Create(ctx, newTask("first"))
	if err != nil || created.ID == 0 {
		t.Fatalf("Create = %+v, %v", created, err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "first" || got.Description != "desc" || got.Completed {
		t.Fatalf("GetByID = %+v", got)
	}
	// PostgreSQL stores microseconds; compare at that precision.
	if !got.CreatedAt.Equal(created.CreatedAt.Truncate(time.Microsecond)) {
		t.Fatalf("CreatedAt round trip: got %v want %v", got.CreatedAt, created.CreatedAt)
	}

	got.Title, got.Completed = "renamed", true
	got.UpdatedAt = time.Now().UTC()
	updated, err := repo.Update(ctx, got)
	if err != nil || updated.Title != "renamed" || !updated.Completed {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	if !updated.CreatedAt.Equal(got.CreatedAt) {
		t.Fatal("Update must not change created_at")
	}
	reread, _ := repo.GetByID(ctx, created.ID)
	if reread.Title != "renamed" || !reread.Completed {
		t.Fatalf("update not persisted: %+v", reread)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestNotFoundMapping(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	if _, err := repo.GetByID(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID: %v", err)
	}
	missing := newTask("ghost")
	missing.ID = 999
	if _, err := repo.Update(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update: %v", err)
	}
	if err := repo.Delete(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ := repo.List(ctx); len(list) != 0 {
		t.Fatal("Update of a missing row must not insert it")
	}
}

func TestListEmptyAndOrdered(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	empty, err := repo.List(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty List = %#v, %v; want non-nil empty slice", empty, err)
	}
	for _, title := range []string{"a", "b", "c"} {
		if _, err := repo.Create(ctx, newTask(title)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("List = %v, %v", list, err)
	}
	for i := 1; i < len(list); i++ {
		if list[i].ID <= list[i-1].ID {
			t.Fatalf("not ordered by id: %v", list)
		}
	}
}

func TestIDsAreNotReusedAfterDelete(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	first, _ := repo.Create(ctx, newTask("first"))
	_ = repo.Delete(ctx, first.ID)
	second, err := repo.Create(ctx, newTask("second"))
	if err != nil || second.ID <= first.ID {
		t.Fatalf("second.ID=%d first.ID=%d err=%v", second.ID, first.ID, err)
	}
}

func TestSQLInjectionPayloadIsStoredLiterally(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	payload := "x'); DROP TABLE tasks;--"
	created, err := repo.Create(ctx, newTask(payload))
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil || got.Title != payload {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := repo.List(ctx); err != nil {
		t.Fatalf("table must still exist: %v", err)
	}
}

func TestDatabaseConstraintRejectsBlankTitle(t *testing.T) {
	// Defence in depth: even if domain validation were bypassed, the CHECK
	// constraint stops blank titles from being persisted.
	repo, ctx := newTestRepo(t), context.Background()
	bad := newTask("ok")
	bad.Title = "   "
	if _, err := repo.Create(ctx, bad); err == nil {
		t.Fatal("want constraint violation error")
	}
	if list, _ := repo.List(ctx); len(list) != 0 {
		t.Fatal("blank-title row must not exist")
	}
}

func TestConcurrentCreateProducesUniqueIDs(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	const n = 50
	ids := make(chan int64, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task, err := repo.Create(ctx, newTask(fmt.Sprintf("t%d", i)))
			if err != nil {
				t.Errorf("Create: %v", err)
				return
			}
			ids <- task.ID
		}(i)
	}
	wg.Wait()
	close(ids)
	seen := map[int64]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %d", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d ids, want %d", len(seen), n)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	repo, ctx := newTestRepo(t), context.Background()
	seed, _ := repo.Create(ctx, newTask("seed"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); _, _ = repo.List(ctx) }()
		go func() { defer wg.Done(); _, _ = repo.GetByID(ctx, seed.ID) }()
		go func() {
			defer wg.Done()
			s := seed
			s.Title, s.UpdatedAt = "upd", time.Now().UTC()
			_, _ = repo.Update(ctx, s)
		}()
	}
	wg.Wait()
	if _, err := repo.GetByID(ctx, seed.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledContextReturnsError(t *testing.T) {
	repo := newTestRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.List(ctx); err == nil {
		t.Fatal("List with canceled context must fail")
	}
	if _, err := repo.Create(ctx, newTask("x")); err == nil {
		t.Fatal("Create with canceled context must fail")
	}
	if err := repo.Delete(ctx, 1); errors.Is(err, domain.ErrNotFound) || err == nil {
		t.Fatalf("Delete must surface the context error, got %v", err)
	}
}

func TestOperationsFailAfterClose(t *testing.T) {
	repo := newTestRepo(t)
	repo.Close()
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatal("List on a closed pool must fail")
	}
}
