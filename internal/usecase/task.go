package usecase

import (
	"context"
	"log/slog"

	"github.com/go-crud/go-task-api/internal/domain"
)

type TaskRepository interface {
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	List(ctx context.Context) ([]domain.Task, error)
	GetByID(ctx context.Context, id int64) (domain.Task, error)
	Update(ctx context.Context, task domain.Task) (domain.Task, error)
	Delete(ctx context.Context, id int64) error
}

type EventPublisher interface {
	Publish(event TaskEvent) bool
}

type TaskEvent struct {
	Type   string
	TaskID int64
}

type TaskService struct {
	repo   TaskRepository
	events EventPublisher
	log    *slog.Logger
}

func NewTask(repo TaskRepository, events EventPublisher, log *slog.Logger) *TaskService {
	return &TaskService{repo: repo, events: events, log: log}
}

func (s *TaskService) Create(ctx context.Context, title, description string) (domain.Task, error) {
	task, err := domain.NewTask(title, description)
	if err != nil {
		return domain.Task{}, err
	}
	created, err := s.repo.Create(ctx, task)
	if err != nil {
		s.log.Error("create task failed", slog.Any("error", err))
		return domain.Task{}, err
	}
	s.publish("task.created", created.ID)
	return created, nil
}

func (s *TaskService) List(ctx context.Context) ([]domain.Task, error) {
	return s.repo.List(ctx)
}

func (s *TaskService) Get(ctx context.Context, id int64) (domain.Task, error) {
	if id <= 0 {
		return domain.Task{}, domain.ErrInvalidID
	}
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) Update(ctx context.Context, id int64, title, description string, completed *bool) (domain.Task, error) {
	if id <= 0 {
		return domain.Task{}, domain.ErrInvalidID
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	if err := task.Update(title, description); err != nil {
		return domain.Task{}, err
	}
	if completed != nil {
		task.Completed = *completed
	}
	updated, err := s.repo.Update(ctx, task)
	if err != nil {
		return domain.Task{}, err
	}
	s.publish("task.updated", updated.ID)
	return updated, nil
}

func (s *TaskService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return domain.ErrInvalidID
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publish("task.deleted", id)
	return nil
}

func (s *TaskService) publish(eventType string, id int64) {
	if s.events != nil && !s.events.Publish(TaskEvent{Type: eventType, TaskID: id}) {
		s.log.Warn("task event dropped", slog.String("event", eventType), slog.Int64("task_id", id))
	}
}
