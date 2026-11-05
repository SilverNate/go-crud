package worker

import (
	"log/slog"
	"sync"

	"github.com/go-crud/go-task-api/internal/usecase"
)

// Worker is a bounded, in-process event consumer. Events are best-effort;
// use a transactional outbox if delivery must survive process restarts.
type Worker struct {
	queue chan usecase.TaskEvent
	log   *slog.Logger
	wg    sync.WaitGroup
	once  sync.Once
}

func New(queueSize int, log *slog.Logger) *Worker {
	if queueSize < 1 {
		queueSize = 1
	}
	return &Worker{queue: make(chan usecase.TaskEvent, queueSize), log: log}
}

func (w *Worker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for event := range w.queue {
			w.log.Info("task domain event",
				slog.String("event_type", event.Type),
				slog.Int64("task_id", event.TaskID),
			)
		}
	}()
}

func (w *Worker) Publish(event usecase.TaskEvent) bool {
	select {
	case w.queue <- event:
		return true
	default:
		return false
	}
}

func (w *Worker) Stop() {
	w.once.Do(func() { close(w.queue) })
	w.wg.Wait()
}
