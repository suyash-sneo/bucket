package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bucket/internal/db"
	"bucket/internal/domain"
	"bucket/internal/service"
	"bucket/internal/ui"
)

func setupServiceForDraftTest(t *testing.T) (*service.TaskService, string) {
	t.Helper()
	home := t.TempDir()
	databasePath := filepath.Join(home, ".config", "bucket", "bucket.db")
	backupsDir := filepath.Join(home, ".config", "bucket", "backups")
	draftsDir := filepath.Join(home, ".config", "bucket", "drafts")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		t.Fatalf("create db dir: %v", err)
	}
	database, err := db.Open(databasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.RunMigrations(context.Background(), database, databasePath, backupsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	taskRepo := db.NewTaskRepo(database)
	subtaskRepo := db.NewSubtaskRepo(database)
	taskService := service.NewTaskService(taskRepo, subtaskRepo, time.Now)
	return taskService, draftsDir
}

func createDraftTestTask(t *testing.T, taskService *service.TaskService) domain.Task {
	t.Helper()
	task, err := taskService.QuickAdd("draft task")
	if err != nil {
		t.Fatalf("quick add: %v", err)
	}
	return task
}

func TestRecoverDraftsAppliesMatchingDraft(t *testing.T) {
	taskService, draftsDir := setupServiceForDraftTest(t)
	task := createDraftTestTask(t, taskService)

	draft := ui.TaskDraft{
		TaskID:        task.ID,
		BaseUpdatedAt: task.UpdatedAt.Unix(),
		SavedAt:       time.Now().UTC().Unix(),
		Fields: ui.TaskDraftFields{
			Title:    "recovered",
			Status:   domain.StatusInProgress,
			URL:      "example.com",
			Notes:    "draft notes",
			Due:      "2026-03-04 10:00",
			Priority: "2",
			Estimate: "90",
			Progress: "50",
		},
		Meta: map[string]any{"extra": "value"},
	}
	path, err := ui.WriteDraftFile(draftsDir, draft)
	if err != nil {
		t.Fatalf("write draft: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	conflicts, err := recoverDrafts(taskService, draftsDir, logger)
	if err != nil {
		t.Fatalf("recover drafts: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %v", conflicts)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected draft file deleted, stat err=%v", err)
	}
	loaded, _, err := taskService.GetDetails(task.ID)
	if err != nil {
		t.Fatalf("get task details: %v", err)
	}
	if loaded.Title != "recovered" || loaded.Notes != "draft notes" {
		t.Fatalf("expected draft fields applied, got %+v", loaded)
	}
}

func TestRecoverDraftsKeepsConflictingDraft(t *testing.T) {
	taskService, draftsDir := setupServiceForDraftTest(t)
	task := createDraftTestTask(t, taskService)

	draft := ui.TaskDraft{
		TaskID:        task.ID,
		BaseUpdatedAt: task.UpdatedAt.Unix() - 1,
		SavedAt:       time.Now().UTC().Unix(),
		Fields: ui.TaskDraftFields{
			Title: "conflicting",
		},
	}
	path, err := ui.WriteDraftFile(draftsDir, draft)
	if err != nil {
		t.Fatalf("write draft: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	conflicts, err := recoverDrafts(taskService, draftsDir, logger)
	if err != nil {
		t.Fatalf("recover drafts: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0] != path {
		t.Fatalf("expected conflict path %s, got %v", path, conflicts)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected draft file to remain, got stat err=%v", err)
	}
}
