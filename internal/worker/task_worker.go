package worker

import (
	"log/slog"
	"sync"

	"github.com/go-crud/go-task-api/internal/usecase"
)

// Worker is a bounded, in-process event consumer. Events are best-effort;
// use a transactional outbox if delivery must survive process restarts.
//
// Lifecycle: producers call Publish concurrently; Stop closes the queue exactly
// once. mu guards the "closed" state so a channel is never closed while a
// producer can still send on it (which would panic).
type Worker struct {
	queue  chan usecase.TaskEvent
	log    *slog.Logger
	wg     sync.WaitGroup
	once   sync.Once
	mu     sync.RWMutex // RLock: publishers; Lock: Stop
	closed bool
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

// Publish never blocks. It returns false if the queue is full or the worker has
// been stopped.
func (w *Worker) Publish(event usecase.TaskEvent) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return false
	}
	select {
	case w.queue <- event:
		return true
	default:
		return false
	}
}

// Stop rejects new events, lets the consumer drain what is already queued, and
// waits for it to exit. It is safe to call more than once.
func (w *Worker) Stop() {
	w.once.Do(func() {
		w.mu.Lock() // waits for in-flight (non-blocking) Publish calls
		w.closed = true
		close(w.queue)
		w.mu.Unlock()
	})
	w.wg.Wait()
}
