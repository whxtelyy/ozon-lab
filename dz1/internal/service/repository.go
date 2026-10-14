package service

import (
	"context"

	"tasktracker/internal/domain"
)

// Repository is declared by the business layer because it describes exactly
// the persistence operations required by the service.
type Repository interface {
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	List(ctx context.Context) ([]domain.Task, error)
	Get(ctx context.Context, id domain.TaskID) (domain.Task, error)
	Update(ctx context.Context, task domain.Task) error
	Delete(ctx context.Context, id domain.TaskID) error
}
