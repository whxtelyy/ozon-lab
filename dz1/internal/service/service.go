package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"tasktracker/internal/domain"
)

var (
	ErrTaskNotFound = domain.ErrTaskNotFound
	ErrSameStatus   = errors.New("task already has this status")
	ErrTaskClosed   = errors.New("task is closed and cannot be changed")
	ErrPageTooBig   = errors.New("page is out of range")
)

type ListFilter struct {
	Status  *domain.Status
	Overdue bool
	Search  string
	Page    int
	Limit   int
}

type ListResult struct {
	Tasks      []domain.Task
	Total      int
	Page       int
	Limit      int
	TotalPages int
}

type TaskInput struct {
	Index    int
	Title    string
	Deadline time.Time
	Status   domain.Status
}

type ImportIssue struct {
	Index int
	Err   error
}

type ImportResult struct {
	Added  []domain.Task
	Issues []ImportIssue
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func New(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func NewWithClock(repo Repository, now func() time.Time) *Service {
	return &Service{
		repo: repo,
		now:  now,
	}
}

func (s *Service) Create(ctx context.Context, title string, deadline time.Time) (domain.Task, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	if err := domain.ValidateDeadline(deadline, s.now()); err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}

	task := domain.Task{
		Title:    domain.NormalizeTitle(title),
		Status:   domain.StatusPlanned,
		Deadline: deadline,
	}

	created, err := s.repo.Create(ctx, task)
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return created, nil
}

func (s *Service) Get(ctx context.Context, id domain.TaskID) (domain.Task, error) {
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("get task %d: %w", id, err)
	}
	return task, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}

	tasks, err := s.repo.List(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("list tasks: %w", err)
	}

	search := strings.ToLower(strings.TrimSpace(filter.Search))
	result := make([]domain.Task, 0)

	for _, task := range tasks {
		if filter.Status != nil && task.Status != *filter.Status {
			continue
		}
		if filter.Overdue && !s.isOverdue(task) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(task.Title), search) {
			continue
		}
		result = append(result, task)
	}

	// Сортируем по дедлайну, а если он одинаковый — по ID.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Deadline.Equal(result[j].Deadline) {
			return result[i].ID < result[j].ID
		}
		return result[i].Deadline.Before(result[j].Deadline)
	})

	total := len(result)
	totalPages := (total + filter.Limit - 1) / filter.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	if filter.Page > totalPages {
		return ListResult{}, fmt.Errorf("list tasks: %w: page %d, total pages %d", ErrPageTooBig, filter.Page, totalPages)
	}

	start := (filter.Page - 1) * filter.Limit
	end := start + filter.Limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return ListResult{
		Tasks:      result[start:end],
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) ChangeStatus(ctx context.Context, id domain.TaskID, newStatus domain.Status) (domain.Task, error) {
	if _, err := domain.ParseStatus(string(newStatus)); err != nil {
		return domain.Task{}, fmt.Errorf("change status: %w", err)
	}

	task, err := s.Get(ctx, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("change status: %w", err)
	}

	if task.Status == domain.StatusDone || task.Status == domain.StatusCanceled {
		return domain.Task{}, fmt.Errorf("change status: %w", ErrTaskClosed)
	}
	if task.Status == newStatus {
		return domain.Task{}, fmt.Errorf("change status: %w", ErrSameStatus)
	}

	task.Status = newStatus
	if err := s.repo.Update(ctx, task); err != nil {
		return domain.Task{}, fmt.Errorf("change status: %w", err)
	}
	return task, nil
}

func (s *Service) Edit(ctx context.Context, id domain.TaskID, title *string, deadline *time.Time) (domain.Task, error) {
	if title == nil && deadline == nil {
		return domain.Task{}, errors.New("edit task: no changes provided")
	}

	task, err := s.Get(ctx, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("edit task: %w", err)
	}

	if task.Status == domain.StatusDone || task.Status == domain.StatusCanceled {
		return domain.Task{}, fmt.Errorf("edit task: %w", ErrTaskClosed)
	}

	if title != nil {
		if err := domain.ValidateTitle(*title); err != nil {
			return domain.Task{}, fmt.Errorf("edit task: %w", err)
		}
		task.Title = domain.NormalizeTitle(*title)
	}

	if deadline != nil {
		if err := domain.ValidateDeadline(*deadline, s.now()); err != nil {
			return domain.Task{}, fmt.Errorf("edit task: %w", err)
		}
		task.Deadline = *deadline
	}

	if err := s.repo.Update(ctx, task); err != nil {
		return domain.Task{}, fmt.Errorf("edit task: %w", err)
	}

	return task, nil
}

func (s *Service) Delete(ctx context.Context, id domain.TaskID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	return nil
}

func (s *Service) Import(ctx context.Context, items []TaskInput) ImportResult {
	result := ImportResult{}

	for i, item := range items {
		index := item.Index
		if index == 0 {
			index = i + 1
		}

		if err := domain.ValidateTitle(item.Title); err != nil {
			result.Issues = append(result.Issues, ImportIssue{Index: index, Err: fmt.Errorf("title: %w", err)})
			continue
		}

		if err := domain.ValidateDeadline(item.Deadline, s.now()); err != nil {
			result.Issues = append(result.Issues, ImportIssue{Index: index, Err: fmt.Errorf("deadline: %w", err)})
			continue
		}

		status := item.Status
		if status == "" {
			status = domain.StatusPlanned
		}
		if _, err := domain.ParseStatus(string(status)); err != nil {
			result.Issues = append(result.Issues, ImportIssue{Index: index, Err: fmt.Errorf("status: %w", err)})
			continue
		}

		task, err := s.repo.Create(ctx, domain.Task{
			Title:    domain.NormalizeTitle(item.Title),
			Status:   status,
			Deadline: item.Deadline,
		})
		if err != nil {
			result.Issues = append(result.Issues, ImportIssue{Index: index, Err: fmt.Errorf("save: %w", err)})
			continue
		}

		result.Added = append(result.Added, task)
	}

	return result
}

func (s *Service) isOverdue(task domain.Task) bool {
	if task.Status == domain.StatusDone || task.Status == domain.StatusCanceled {
		return false
	}

	return errors.Is(domain.ValidateDeadline(task.Deadline, s.now()), domain.ErrPastDeadline)
}
