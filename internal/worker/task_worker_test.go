package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/go-crud/go-task-api/internal/usecase"
)

// recordHandler is a slog.Handler that captures records for assertions.
type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordHandler) taskIDs() []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var ids []int64
	for _, r := range h.records {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "task_id" {
				ids = append(ids, a.Value.Int64())
			}
			return true
		})
	}
	return ids
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPublishAcceptedAndConsumed(t *testing.T) {
	rec := &recordHandler{}
	w := New(8, slog.New(rec))
	w.Start()

	for i := int64(1); i <= 5; i++ {
		if !w.Publish(usecase.TaskEvent{Type: "task.created", TaskID: i}) {
			t.Fatalf("Publish(%d) = false, want true", i)
		}
	}
	w.Stop() // Stop drains the queue before returning.

	ids := rec.taskIDs()
	if len(ids) != 5 {
		t.Fatalf("consumed %d events, want 5 (ids=%v)", len(ids), ids)
	}
	for i, id := range ids {
		if id != int64(i+1) {
			t.Fatalf("event %d has task_id %d, want %d (FIFO order)", i, id, i+1)
		}
	}
}

func TestPublishReturnsFalseWhenQueueFull(t *testing.T) {
	// Worker is intentionally NOT started, so nothing drains the queue.
	w := New(2, discardLogger())

	if !w.Publish(usecase.TaskEvent{TaskID: 1}) || !w.Publish(usecase.TaskEvent{TaskID: 2}) {
		t.Fatal("first two publishes should fit in the queue")
	}
	done := make(chan bool, 1)
	go func() { done <- w.Publish(usecase.TaskEvent{TaskID: 3}) }()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("Publish on a full queue = true, want false")
		}
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full queue; it must be non-blocking")
	}
}

func TestNewClampsQueueSize(t *testing.T) {
	w := New(0, discardLogger())
	if cap(w.queue) != 1 {
		t.Fatalf("queue capacity = %d, want 1", cap(w.queue))
	}
}

func TestStopIsIdempotent(t *testing.T) {
	w := New(4, discardLogger())
	w.Start()
	w.Stop()
	w.Stop() // must not panic on double close
}

func TestStopWithoutStartDoesNotHang(t *testing.T) {
	w := New(4, discardLogger())
	finished := make(chan struct{})
	go func() { w.Stop(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Stop() hung when Start() was never called")
	}
}

func TestPublishAfterStopReturnsFalseAndDoesNotPanic(t *testing.T) {
	w := New(4, discardLogger())
	w.Start()
	w.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Publish after Stop panicked: %v", r)
		}
	}()
	if w.Publish(usecase.TaskEvent{TaskID: 1}) {
		t.Fatal("Publish after Stop = true, want false")
	}
}

// Regression test for "send on closed channel": many producers race with Stop.
// Run with -race.
func TestConcurrentPublishersWithStop(t *testing.T) {
	w := New(16, discardLogger())
	w.Start()

	const producers = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			<-start
			for j := 0; j < 200; j++ {
				w.Publish(usecase.TaskEvent{Type: "task.created", TaskID: id})
			}
		}(int64(i))
	}
	close(start)
	time.Sleep(time.Millisecond) // let some publishes land before Stop
	w.Stop()
	wg.Wait()
}

func TestConsumerGoroutineExitsAfterStop(t *testing.T) {
	w := New(4, discardLogger())
	w.Start()
	w.Publish(usecase.TaskEvent{TaskID: 1})

	finished := make(chan struct{})
	go func() { w.Stop(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return: consumer goroutine leaked")
	}
}
