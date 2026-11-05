package memory

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"sync"

	"github.com/go-crud/go-task-api/internal/domain"
)

type Repository struct {
	mu     sync.RWMutex
	tasks  map[int64]domain.Task
	nextID int64
}

func New() *Repository {
	return &Repository{tasks: make(map[int64]domain.Task), nextID: 1}
}

// NewFromJSON loads development fixture data into the in-memory repository.
// IDs are assigned locally and are not read from the fixture.
func NewFromJSON(path string) (*Repository, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tasks []domain.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	repo := New()
	for _, task := range tasks {
		if task.Title == "" {
			continue
		}
		task.ID = repo.nextID
		repo.nextID++
		repo.tasks[task.ID] = task
	}
	return repo, nil
}

func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task.ID = r.nextID
	r.nextID++
	r.tasks[task.ID] = task
	return task, nil
}

func (r *Repository) List(ctx context.Context) ([]domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, nil
}

func (r *Repository) Update(ctx context.Context, task domain.Task) (domain.Task, error) {
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[task.ID]; !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}
