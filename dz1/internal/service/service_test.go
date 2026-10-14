package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"tasktracker/internal/domain"
	"tasktracker/internal/storage"
)

func testNow() time.Time {
	return time.Date(2026, time.September, 30, 12, 0, 0, 0, time.Local)
}

func newTestService() *Service {
	return NewWithClock(storage.NewMemoryStore(), testNow)
}

func TestCreateAssignsMonotonicIDs(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	first, err := s.Create(ctx, "first", testNow().AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(ctx, "second", testNow().AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != 1 || second.ID != 2 {
		t.Fatalf("unexpected IDs: %d, %d", first.ID, second.ID)
	}
	if err := s.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	third, err := s.Create(ctx, "third", testNow().AddDate(0, 0, 3))
	if err != nil {
		t.Fatal(err)
	}
	if third.ID != 3 {
		t.Fatalf("ID was reused: got %d, want 3", third.ID)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	if _, err := s.Create(ctx, "   ", testNow().AddDate(0, 0, 1)); !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatalf("invalid title error: %v", err)
	}
	if _, err := s.Create(ctx, "past", testNow().AddDate(0, 0, -1)); !errors.Is(err, domain.ErrPastDeadline) {
		t.Fatalf("past deadline error: %v", err)
	}
}

func TestListSortsAndFilters(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	deadline := testNow().AddDate(0, 0, 1)
	if _, err := s.Create(ctx, "Beta", deadline); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "alpha", deadline); err != nil {
		t.Fatal(err)
	}
	third, err := s.Create(ctx, "Old task", deadline)
	if err != nil {
		t.Fatal(err)
	}
	third.Deadline = testNow().AddDate(0, 0, -1)
	third.Status = domain.StatusInProgress
	if err := s.repo.Update(ctx, third); err != nil {
		t.Fatal(err)
	}

	result, err := s.List(ctx, ListFilter{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 3 {
		t.Fatalf("got %d tasks", len(result.Tasks))
	}
	if result.Tasks[0].ID != third.ID {
		t.Fatalf("sort by deadline failed, first id=%d", result.Tasks[0].ID)
	}
	if result.Tasks[1].ID != 1 || result.Tasks[2].ID != 2 {
		t.Fatalf("tie-break by ID failed: %+v", result.Tasks)
	}

	status := domain.StatusInProgress
	result, err = s.List(ctx, ListFilter{Status: &status, Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Old task" {
		t.Fatalf("status filter failed: %+v", result)
	}

	result, err = s.List(ctx, ListFilter{Overdue: true, Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Title != "Old task" {
		t.Fatalf("overdue filter failed: %+v", result)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	_, err := s.Create(ctx, "Подготовить отчет", testNow().AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, "Купить молоко", testNow().AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, "ОТЧЕТ для пары", testNow().AddDate(0, 0, 3))
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.List(ctx, ListFilter{
		Search: "ОТЧЕТ",
		Page:   1,
		Limit:  20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Tasks) != 2 {
		t.Fatalf("search failed: %+v", result)
	}
}

func TestChangeStatusRules(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	task, err := s.Create(ctx, "task", testNow().AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ChangeStatus(ctx, task.ID, domain.StatusPlanned); !errors.Is(err, ErrSameStatus) {
		t.Fatalf("same status error: %v", err)
	}
	if _, err := s.ChangeStatus(ctx, task.ID, domain.StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeStatus(ctx, task.ID, domain.StatusDone); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangeStatus(ctx, task.ID, domain.StatusPlanned); !errors.Is(err, ErrTaskClosed) {
		t.Fatalf("closed task error: %v", err)
	}
}

func TestEditRules(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	task, err := s.Create(ctx, "old", testNow().AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}

	newTitle := "new"
	newDeadline := testNow().AddDate(0, 0, 5)
	updated, err := s.Edit(ctx, task.ID, &newTitle, &newDeadline)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != newTitle || !updated.Deadline.Equal(newDeadline) {
		t.Fatalf("edit failed: %+v", updated)
	}

	if _, err := s.Edit(ctx, task.ID, nil, nil); err == nil {
		t.Fatal("expected no changes error")
	}
	if _, err := s.ChangeStatus(ctx, task.ID, domain.StatusDone); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit(ctx, task.ID, &newTitle, nil); !errors.Is(err, ErrTaskClosed) {
		t.Fatalf("closed edit error: %v", err)
	}
}

func TestListPagination(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := s.Create(ctx, fmt.Sprintf("task %d", i+1), testNow().AddDate(0, 0, i+1))
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := s.List(ctx, ListFilter{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || result.TotalPages != 2 || len(result.Tasks) != 2 {
		t.Fatalf("unexpected page result: %+v", result)
	}

	result, err = s.List(ctx, ListFilter{Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].ID != 3 {
		t.Fatalf("unexpected second page: %+v", result)
	}

	if _, err := s.List(ctx, ListFilter{Page: 3, Limit: 2}); !errors.Is(err, ErrPageTooBig) {
		t.Fatalf("expected page error, got %v", err)
	}

	result, err = s.List(ctx, ListFilter{Page: 1, Limit: 20, Search: "nothing"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 || result.TotalPages != 1 {
		t.Fatalf("unexpected empty result: %+v", result)
	}
}

func TestImportAddsValidTasksAndSkipsInvalidOnes(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	items := []TaskInput{
		{
			Index:    1,
			Title:    "valid one",
			Status:   domain.StatusPlanned,
			Deadline: testNow().AddDate(0, 0, 1),
		},
		{
			Index:    2,
			Title:    "   ",
			Status:   domain.StatusPlanned,
			Deadline: testNow().AddDate(0, 0, 2),
		},
		{
			Index:    3,
			Title:    "valid two",
			Status:   domain.StatusInProgress,
			Deadline: testNow().AddDate(0, 0, 3),
		},
		{
			Index:    4,
			Title:    "bad status",
			Status:   domain.Status("unknown"),
			Deadline: testNow().AddDate(0, 0, 4),
		},
		{
			Index:    5,
			Title:    "past deadline",
			Status:   domain.StatusPlanned,
			Deadline: testNow().AddDate(0, 0, -1),
		},
	}

	result := s.Import(ctx, items)
	if len(result.Added) != 2 {
		t.Fatalf("added %d tasks, want 2", len(result.Added))
	}
	if len(result.Issues) != 3 {
		t.Fatalf("got %d import errors, want 3", len(result.Issues))
	}
	if result.Added[0].Title != "valid one" || result.Added[1].Title != "valid two" {
		t.Fatalf("unexpected imported tasks: %+v", result.Added)
	}
	if result.Issues[0].Index != 2 || result.Issues[1].Index != 4 || result.Issues[2].Index != 5 {
		t.Fatalf("unexpected issue indexes: %+v", result.Issues)
	}

	list, err := s.List(ctx, ListFilter{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 {
		t.Fatalf("repository contains %d tasks after import, want 2", list.Total)
	}
}
