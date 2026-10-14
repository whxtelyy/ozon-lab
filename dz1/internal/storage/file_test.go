package storage

import (
	"context"
	"testing"
	"time"

	"tasktracker/internal/domain"
)

func TestFileStorePersistsAndDoesNotReuseIDs(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	deadline := time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	ctx := context.Background()

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(ctx, domain.Task{
		Title:    "first",
		Status:   domain.StatusPlanned,
		Deadline: deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, domain.Task{
		Title:    "second",
		Status:   domain.StatusPlanned,
		Deadline: deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ID != second.ID {
		t.Fatalf("unexpected loaded tasks: %+v", loaded)
	}

	third, err := reopened.Create(ctx, domain.Task{
		Title:    "third",
		Status:   domain.StatusPlanned,
		Deadline: deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	if third.ID != 3 {
		t.Fatalf("expected ID 3 after restart, got %d", third.ID)
	}
}
