package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type TaskID int64

type Status string

const (
	StatusPlanned    Status = "planned"
	StatusInProgress Status = "in_progress"
	StatusCanceled   Status = "canceled"
	StatusDone       Status = "done"
)

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrInvalidTitle  = errors.New("invalid title")
	ErrPastDeadline  = errors.New("deadline is in the past")
	ErrInvalidStatus = errors.New("invalid status")
)

type Task struct {
	ID       TaskID    `json:"id"`
	Title    string    `json:"title"`
	Status   Status    `json:"status"`
	Deadline time.Time `json:"deadline"`
}

func ParseStatus(value string) (Status, error) {
	status := Status(value)
	switch status {
	case StatusPlanned, StatusInProgress, StatusCanceled, StatusDone:
		return status, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidStatus, value)
	}
}

func ValidateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	return nil
}

func ValidateDeadline(deadline, now time.Time) error {
	location := now.Location()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if deadline.Before(startOfToday) {
		return ErrPastDeadline
	}
	return nil
}

func NormalizeTitle(title string) string {
	return strings.TrimSpace(title)
}

func FormatDeadline(deadline time.Time) string {
	return deadline.Format("2006-01-02")
}

func ParseDeadline(value string, location *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, location)
}
