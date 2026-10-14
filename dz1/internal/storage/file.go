package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"tasktracker/internal/domain"
)

type FileStore struct {
	mu       sync.RWMutex
	path     string
	nextID   domain.TaskID
	tasks    []domain.Task
	location *time.Location
}

type fileData struct {
	NextID domain.TaskID `json:"next_id"`
	Tasks  []fileTask    `json:"tasks"`
}

type fileTask struct {
	ID       domain.TaskID `json:"id"`
	Title    string        `json:"title"`
	Status   domain.Status `json:"status"`
	Deadline string        `json:"deadline"`
}

func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:     path,
		nextID:   1,
		location: time.Local,
	}

	if err := store.load(); err != nil {
		return nil, fmt.Errorf("create file store: %w", err)
	}

	return store, nil
}

func (s *FileStore) Create(_ context.Context, task domain.Task) (domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID != 0 {
		return domain.Task{}, errors.New("file store: task ID must be zero")
	}

	task.ID = s.nextID
	s.nextID++
	s.tasks = append(s.tasks, task)

	if err := s.save(); err != nil {
		s.tasks = s.tasks[:len(s.tasks)-1]
		s.nextID--
		return domain.Task{}, fmt.Errorf("file create: %w", err)
	}

	return task, nil
}

func (s *FileStore) List(_ context.Context) ([]domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]domain.Task(nil), s.tasks...), nil
}

func (s *FileStore) Get(_ context.Context, id domain.TaskID) (domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.tasks {
		if task.ID == id {
			return task, nil
		}
	}

	return domain.Task{}, fmt.Errorf("file get %d: %w", id, domain.ErrTaskNotFound)
}

func (s *FileStore) Update(_ context.Context, task domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID != task.ID {
			continue
		}

		old := s.tasks[i]
		s.tasks[i] = task

		if err := s.save(); err != nil {
			s.tasks[i] = old
			return fmt.Errorf("file update %d: %w", task.ID, err)
		}

		return nil
	}

	return fmt.Errorf("file update %d: %w", task.ID, domain.ErrTaskNotFound)
}

func (s *FileStore) Delete(_ context.Context, id domain.TaskID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID != id {
			continue
		}

		old := s.tasks[i]
		s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)

		if err := s.save(); err != nil {
			s.tasks = append(s.tasks[:i], append([]domain.Task{old}, s.tasks[i:]...)...)
			return fmt.Errorf("file delete %d: %w", id, err)
		}

		return nil
	}

	return fmt.Errorf("file delete %d: %w", id, domain.ErrTaskNotFound)
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var stored fileData
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}

	maxID := domain.TaskID(0)
	tasks := make([]domain.Task, 0, len(stored.Tasks))
	seen := make(map[domain.TaskID]bool)

	for i, item := range stored.Tasks {
		if item.ID <= 0 {
			return fmt.Errorf("validate task %d: invalid id %d", i+1, item.ID)
		}
		if seen[item.ID] {
			return fmt.Errorf("validate task %d: duplicate id %d", i+1, item.ID)
		}
		seen[item.ID] = true

		status, err := domain.ParseStatus(string(item.Status))
		if err != nil {
			return fmt.Errorf("validate task %d status: %w", i+1, err)
		}
		if err := domain.ValidateTitle(item.Title); err != nil {
			return fmt.Errorf("validate task %d title: %w", i+1, err)
		}

		deadline, err := domain.ParseDeadline(item.Deadline, s.location)
		if err != nil {
			return fmt.Errorf("validate task %d deadline: %w", i+1, err)
		}

		if item.ID > maxID {
			maxID = item.ID
		}

		tasks = append(tasks, domain.Task{
			ID:       item.ID,
			Title:    domain.NormalizeTitle(item.Title),
			Status:   status,
			Deadline: deadline,
		})
	}

	s.tasks = tasks
	s.nextID = stored.NextID
	if s.nextID <= maxID {
		s.nextID = maxID + 1
	}
	if s.nextID <= 0 {
		s.nextID = 1
	}

	return nil
}

func (s *FileStore) save() error {
	stored := fileData{
		NextID: s.nextID,
		Tasks:  make([]fileTask, 0, len(s.tasks)),
	}

	for _, task := range s.tasks {
		stored.Tasks = append(stored.Tasks, fileTask{
			ID:       task.ID,
			Title:    task.Title,
			Status:   task.Status,
			Deadline: domain.FormatDeadline(task.Deadline),
		})
	}

	sort.Slice(stored.Tasks, func(i, j int) bool {
		return stored.Tasks[i].ID < stored.Tasks[j].ID
	})

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write data file: %w", err)
	}

	return nil
}
