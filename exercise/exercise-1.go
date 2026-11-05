package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Task struct {
	ID          int64
	Title       string
	Description string
}

type TaskStore struct {
	mu     sync.RWMutex
	tasks  map[int64]Task
	nextID int64
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks:  make(map[int64]Task),
		nextID: 1,
	}
}

func (s *TaskStore) CreateTask(title, description string) (Task, error) {
	if strings.TrimSpace(title) == "" {
		return Task{}, errors.New("error title is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := Task{
		ID:          s.nextID,
		Title:       title,
		Description: description,
	}

	s.tasks[task.ID] = task
	s.nextID++

	return task, nil
}

func (s *TaskStore) GetTaskByID(id int64) (Task, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	if id <= 0 {
		return Task{}, errors.New("id cannot zero or lower")
	}

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task with ID %d not found", id)
	}
	return task, nil
}

func (s *TaskStore) UpdateTask(id int64, title, description string) (Task, error) {
	// 1. Reject IDs less than or equal to zero
	if id <= 0 {
		return Task{}, errors.New("invalid task ID: ID must be positive")
	}

	// 2. Reject empty or whitespace-only titles
	if strings.TrimSpace(title) == "" {
		return Task{}, errors.New("title cannot be empty or contain only spaces")
	}

	// 6. Acquire write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. Return an error if the task doesn't exist
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task with ID %d not found", id)
	}

	// 4 & 5. Update title and description while preserving original ID
	task.Title = title
	task.Description = description

	s.tasks[id] = task

	// 7. Return the updated task
	return task, nil
}

// DeleteTask validates the ID, acquires a write lock, and removes a task from the map.
func (s *TaskStore) DeleteTask(id int64) error {
	// Reject IDs less than or equal to zero
	if id <= 0 {
		return errors.New("invalid task ID: ID must be positive")
	}

	// Acquire write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return error if task doesn't exist
	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task with ID %d not found", id)
	}

	// Delete task from map (nextID remains untouched)
	delete(s.tasks, id)

	return nil
}

func main() {
	store := NewTaskStore()

	// 1. Create a task
	task1, err := store.CreateTask("Learn Go", "Complete the in-memory task store exercise")
	if err != nil {
		fmt.Println("Error creating task:", err)
		return
	}
	fmt.Printf("Created Task: %+v\n", task1)

	// 2. Retrieve the task
	retrieved, err := store.GetTaskByID(task1.ID)
	if err != nil {
		fmt.Println("Error retrieving task:", err)
		return
	}
	fmt.Printf("Retrieved Task: %+v\n", retrieved)

	// 3. Retrieve a missing task to test error handling
	_, err = store.GetTaskByID(999)
	if err != nil {
		fmt.Println("Expected Error:", err)
	}

	updateTask, err := store.UpdateTask(1, "ganti title", "mengganti title")
	if err != nil {
		fmt.Println("Error Updating task: ", err)
	}
	fmt.Printf("Update Task: %+v\n", updateTask)

	err = store.DeleteTask(1)
	if err != nil {
		fmt.Println("Error Deleting task: ", err)
	}
	fmt.Println("success delete with id 1")

}
