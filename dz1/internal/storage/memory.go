package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"tasktracker/internal/domain"
)

type MemoryStore struct {
	mu     sync.RWMutex
	nextID domain.TaskID
	tasks  []domain.Task
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1}
}

func (s *MemoryStore) Create(_ context.Context, task domain.Task) (domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID != 0 {
		return domain.Task{}, errors.New("memory store: task ID must be zero")
	}

	task.ID = s.nextID
	s.nextID++
	s.tasks = append(s.tasks, task)
	return task, nil
}

func (s *MemoryStore) List(_ context.Context) ([]domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]domain.Task(nil), s.tasks...), nil
}

func (s *MemoryStore) Get(_ context.Context, id domain.TaskID) (domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.tasks {
		if task.ID == id {
			return task, nil
		}
	}

	return domain.Task{}, fmt.Errorf("memory get %d: %w", id, domain.ErrTaskNotFound)
}

func (s *MemoryStore) Update(_ context.Context, task domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID == task.ID {
			s.tasks[i] = task
			return nil
		}
	}

	return fmt.Errorf("memory update %d: %w", task.ID, domain.ErrTaskNotFound)
}

func (s *MemoryStore) Delete(_ context.Context, id domain.TaskID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("memory delete %d: %w", id, domain.ErrTaskNotFound)
}
