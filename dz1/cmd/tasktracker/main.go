package main

import (
	"context"
	"fmt"
	"os"

	"tasktracker/internal/cli"
	"tasktracker/internal/service"
	"tasktracker/internal/storage"
)

func main() {
	// Чтобы переключиться на хранилище в памяти, замените следующую строку на:
	// repo := service.Repository(storage.NewMemoryStore())
	fileStore, err := storage.NewFileStore("tasks.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var repo service.Repository = fileStore

	tracker := service.New(repo)
	app := cli.New(tracker, os.Stdin, os.Stdout)
	if err := app.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
