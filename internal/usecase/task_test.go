package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/go-crud/go-task-api/internal/domain"
)

// fakeRepo is a hand-written fake: function fields let each test override
// exactly the behaviour it cares about, and calls are counted.
type fakeRepo struct {
	createFn  func(context.Context, domain.Task) (domain.Task, error)
	listFn    func(context.Context) ([]domain.Task, error)
	getFn     func(context.Context, int64) (domain.Task, error)
	updateFn  func(context.Context, domain.Task) (domain.Task, error)
	deleteFn  func(context.Context, int64) error
	calls     map[string]int
	lastSaved domain.Task
}

func newFakeRepo() *fakeRepo { return &fakeRepo{calls: map[string]int{}} }

func (f *fakeRepo) Create(ctx context.Context, t domain.Task) (domain.Task, error) {
	f.calls["create"]++
	if f.createFn != nil {
		return f.createFn(ctx, t)
	}
	t.ID = 1
	return t, nil
}
func (f *fakeRepo) List(ctx context.Context) ([]domain.Task, error) {
	f.calls["list"]++
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}
func (f *fakeRepo) GetByID(ctx context.Context, id int64) (domain.Task, error) {
	f.calls["get"]++
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return domain.Task{ID: id, Title: "existing", Description: "d"}, nil
}
func (f *fakeRepo) Update(ctx context.Context, t domain.Task) (domain.Task, error) {
	f.calls["update"]++
	f.lastSaved = t
	if f.updateFn != nil {
		return f.updateFn(ctx, t)
	}
	return t, nil
}
func (f *fakeRepo) Delete(ctx context.Context, id int64) error {
	f.calls["delete"]++
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

type fakePublisher struct {
	events []TaskEvent
	accept bool
}

func (p *fakePublisher) Publish(e TaskEvent) bool {
	p.events = append(p.events, e)
	return p.accept
}

func newService(repo *fakeRepo, pub EventPublisher) *TaskService {
	return NewTask(repo, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var errBoom = errors.New("boom")

func TestCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("success persists and publishes task.created", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		repo.createFn = func(_ context.Context, task domain.Task) (domain.Task, error) {
			task.ID = 42
			return task, nil
		}
		got, err := newService(repo, pub).Create(ctx, "  Title ", "desc")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != 42 || got.Title != "Title" {
			t.Fatalf("got %+v", got)
		}
		if len(pub.events) != 1 || pub.events[0] != (TaskEvent{Type: "task.created", TaskID: 42}) {
			t.Fatalf("events = %+v", pub.events)
		}
	})

	t.Run("blank title rejected before touching repo or events", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		_, err := newService(repo, pub).Create(ctx, "  ", "d")
		if !errors.Is(err, domain.ErrInvalidTitle) {
			t.Fatalf("error = %v", err)
		}
		if repo.calls["create"] != 0 || len(pub.events) != 0 {
			t.Fatal("repo/events must not be called on validation failure")
		}
	})

	t.Run("repository error propagated, no event", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		repo.createFn = func(context.Context, domain.Task) (domain.Task, error) { return domain.Task{}, errBoom }
		_, err := newService(repo, pub).Create(ctx, "t", "d")
		if !errors.Is(err, errBoom) {
			t.Fatalf("error = %v", err)
		}
		if len(pub.events) != 0 {
			t.Fatal("no event may be published when persistence fails")
		}
	})

	t.Run("dropped event does not fail the operation", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: false}
		if _, err := newService(repo, pub).Create(ctx, "t", "d"); err != nil {
			t.Fatalf("error = %v, want nil (events are best-effort)", err)
		}
		if len(pub.events) != 1 {
			t.Fatal("publish should still be attempted")
		}
	})

	t.Run("nil publisher is tolerated", func(t *testing.T) {
		if _, err := newService(newFakeRepo(), nil).Create(ctx, "t", "d"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestList(t *testing.T) {
	repo := newFakeRepo()
	want := []domain.Task{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}}
	repo.listFn = func(context.Context) ([]domain.Task, error) { return want, nil }
	got, err := newService(repo, nil).List(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}

	repo.listFn = func(context.Context) ([]domain.Task, error) { return nil, errBoom }
	if _, err := newService(repo, nil).List(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v", err)
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	for _, id := range []int64{0, -1} {
		repo := newFakeRepo()
		if _, err := newService(repo, nil).Get(ctx, id); !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("id %d: error = %v", id, err)
		}
		if repo.calls["get"] != 0 {
			t.Fatalf("id %d: repo must not be queried for invalid IDs", id)
		}
	}

	repo := newFakeRepo()
	repo.getFn = func(context.Context, int64) (domain.Task, error) { return domain.Task{}, domain.ErrNotFound }
	if _, err := newService(repo, nil).Get(ctx, 5); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}

	repo = newFakeRepo()
	got, err := newService(repo, nil).Get(ctx, 5)
	if err != nil || got.ID != 5 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	yes := true

	t.Run("success applies fields and completed flag, publishes task.updated", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		got, err := newService(repo, pub).Update(ctx, 3, " New ", " desc ", &yes)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "New" || got.Description != "desc" || !got.Completed {
			t.Fatalf("got %+v", got)
		}
		if len(pub.events) != 1 || pub.events[0] != (TaskEvent{Type: "task.updated", TaskID: 3}) {
			t.Fatalf("events = %+v", pub.events)
		}
	})

	t.Run("nil completed leaves existing value untouched", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getFn = func(_ context.Context, id int64) (domain.Task, error) {
			return domain.Task{ID: id, Title: "t", Completed: true}, nil
		}
		got, err := newService(repo, nil).Update(ctx, 1, "t2", "", nil)
		if err != nil || !got.Completed {
			t.Fatalf("got %+v, %v; Completed must stay true", got, err)
		}
	})

	t.Run("explicit false un-completes a task", func(t *testing.T) {
		no := false
		repo := newFakeRepo()
		repo.getFn = func(_ context.Context, id int64) (domain.Task, error) {
			return domain.Task{ID: id, Title: "t", Completed: true}, nil
		}
		got, _ := newService(repo, nil).Update(ctx, 1, "t", "", &no)
		if got.Completed {
			t.Fatal("completed=false must be honoured")
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		repo := newFakeRepo()
		if _, err := newService(repo, nil).Update(ctx, 0, "t", "d", nil); !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("error = %v", err)
		}
		if repo.calls["get"] != 0 {
			t.Fatal("repo must not be queried")
		}
	})

	t.Run("not found skips update and event", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		repo.getFn = func(context.Context, int64) (domain.Task, error) { return domain.Task{}, domain.ErrNotFound }
		if _, err := newService(repo, pub).Update(ctx, 9, "t", "d", nil); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("error = %v", err)
		}
		if repo.calls["update"] != 0 || len(pub.events) != 0 {
			t.Fatal("update/event must not happen")
		}
	})

	t.Run("blank title rejected, nothing saved", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		if _, err := newService(repo, pub).Update(ctx, 1, " ", "d", nil); !errors.Is(err, domain.ErrInvalidTitle) {
			t.Fatalf("error = %v", err)
		}
		if repo.calls["update"] != 0 || len(pub.events) != 0 {
			t.Fatal("update/event must not happen")
		}
	})

	t.Run("repository update error, no event", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		repo.updateFn = func(context.Context, domain.Task) (domain.Task, error) { return domain.Task{}, errBoom }
		if _, err := newService(repo, pub).Update(ctx, 1, "t", "d", nil); !errors.Is(err, errBoom) {
			t.Fatalf("error = %v", err)
		}
		if len(pub.events) != 0 {
			t.Fatal("no event on failure")
		}
	})

	t.Run("row deleted between read and write surfaces ErrNotFound", func(t *testing.T) {
		repo := newFakeRepo()
		repo.updateFn = func(context.Context, domain.Task) (domain.Task, error) { return domain.Task{}, domain.ErrNotFound }
		if _, err := newService(repo, nil).Update(ctx, 1, "t", "d", nil); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("success publishes task.deleted", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		if err := newService(repo, pub).Delete(ctx, 8); err != nil {
			t.Fatal(err)
		}
		if len(pub.events) != 1 || pub.events[0] != (TaskEvent{Type: "task.deleted", TaskID: 8}) {
			t.Fatalf("events = %+v", pub.events)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		repo := newFakeRepo()
		if err := newService(repo, nil).Delete(ctx, -5); !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("error = %v", err)
		}
		if repo.calls["delete"] != 0 {
			t.Fatal("repo must not be called")
		}
	})

	t.Run("not found, no event", func(t *testing.T) {
		repo, pub := newFakeRepo(), &fakePublisher{accept: true}
		repo.deleteFn = func(context.Context, int64) error { return domain.ErrNotFound }
		if err := newService(repo, pub).Delete(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("error = %v", err)
		}
		if len(pub.events) != 0 {
			t.Fatal("no event on failure")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo := newFakeRepo()
		repo.deleteFn = func(context.Context, int64) error { return errBoom }
		if err := newService(repo, nil).Delete(ctx, 1); !errors.Is(err, errBoom) {
			t.Fatalf("error = %v", err)
		}
	})
}
