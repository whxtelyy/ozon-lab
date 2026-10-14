package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"tasktracker/internal/service"
	"tasktracker/internal/storage"
)

func TestListShowsPageError(t *testing.T) {
	tracker := service.New(storage.NewMemoryStore())
	_, err := tracker.Create(context.Background(), "test", testDeadline())
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	app := New(tracker, strings.NewReader("list --page 2 --limit 10\nexit\n"), &out)
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "page is out of range") {
		t.Fatalf("expected page error, got:\n%s", out.String())
	}
}

func TestImportContinuesAfterInvalidJSONItem(t *testing.T) {
	tracker := service.New(storage.NewMemoryStore())
	path := t.TempDir() + "/import.json"

	data := `[
		{"title":"one","status":"planned","deadline":"2099-10-10"},
		{"title":"broken","status":123,"deadline":"2099-10-11"},
		{"title":"two","status":"in_progress","deadline":"2099-10-12"}
	]`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	app := New(tracker, strings.NewReader("import "+path+"\nlist\nexit\n"), &out)
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, "Импорт завершен: добавлено 2, ошибок 1.") {
		t.Fatalf("unexpected import output:\n%s", output)
	}
	if !strings.Contains(output, "Элемент 2:") {
		t.Fatalf("expected invalid item information:\n%s", output)
	}
	if !strings.Contains(output, "Найдено: 2") {
		t.Fatalf("expected two valid tasks:\n%s", output)
	}
}

func testDeadline() time.Time {
	return time.Date(2099, 10, 10, 0, 0, 0, 0, time.Local)
}

func TestTokenizeKeepsWindowsPath(t *testing.T) {
	args, err := tokenize(`import C:\Users\saveliy\AppData\Local\Temp\import.json`)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"import", `C:\Users\saveliy\AppData\Local\Temp\import.json`}
	if len(args) != len(expected) {
		t.Fatalf("expected %d arguments, got %d: %#v", len(expected), len(args), args)
	}

	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("argument %d: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestTokenizeWindowsPathInQuotes(t *testing.T) {
	args, err := tokenize(`import "C:\Users\saveliy\My Files\import.json"`)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"import", `C:\Users\saveliy\My Files\import.json`}
	if len(args) != len(expected) {
		t.Fatalf("expected %d arguments, got %d: %#v", len(expected), len(args), args)
	}

	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("argument %d: expected %q, got %q", i, expected[i], args[i])
		}
	}
}
