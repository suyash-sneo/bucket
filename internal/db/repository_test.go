package db

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bucket/internal/domain"
)

func setupInMemoryDB(t *testing.T) (*sql.DB, *SQLiteTaskRepo, *SQLiteSubtaskRepo) {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := RunMigrations(context.Background(), database, ":memory:", t.TempDir()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return database, NewTaskRepo(database), NewSubtaskRepo(database)
}

func createTask(t *testing.T, repo *SQLiteTaskRepo, title, status string, due *time.Time) domain.Task {
	t.Helper()
	now := time.Now().UTC()
	task, err := repo.CreateTask(context.Background(), domain.Task{
		Title:     title,
		Status:    status,
		DueAt:     due,
		Meta:      map[string]any{"x": "y"},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create task %q: %v", title, err)
	}
	return task
}

func setTaskCreatedAt(t *testing.T, repo *SQLiteTaskRepo, taskID int64, createdAt time.Time) {
	t.Helper()
	if _, err := repo.db.Exec(
		"UPDATE tasks SET created_at = ?, updated_at = ? WHERE id = ?",
		createdAt.UTC().Unix(),
		createdAt.UTC().Unix(),
		taskID,
	); err != nil {
		t.Fatalf("set task created_at for task %d: %v", taskID, err)
	}
}

func TestMigrationsCreateSchemaInMemory(t *testing.T) {
	database, _, _ := setupInMemoryDB(t)
	rows, err := database.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables[name] = true
	}
	for _, name := range []string{"tasks", "subtasks", "schema_migrations"} {
		if !tables[name] {
			t.Fatalf("expected table %s", name)
		}
	}
}

func TestTaskCRUDAndOptionalFields(t *testing.T) {
	_, taskRepo, _ := setupInMemoryDB(t)
	task := createTask(t, taskRepo, "title-only", domain.StatusCreated, nil)

	due := time.Now().UTC().Add(2 * time.Hour)
	priority := 3
	estimate := 90
	progress := 45
	task.Title = "updated"
	task.URL = "example.com"
	task.Notes = "notes"
	task.DueAt = &due
	task.Priority = &priority
	task.EstimatedMinutes = &estimate
	task.Progress = &progress
	task.Meta = map[string]any{"known": "value", "unknown": map[string]any{"nested": true}}
	base := task.UpdatedAt
	task.UpdatedAt = time.Now().UTC()

	updated, err := taskRepo.UpdateTask(context.Background(), base, task)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	loaded, _, err := taskRepo.GetTask(context.Background(), updated.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if loaded.Title != "updated" || loaded.URL != "https://example.com" || loaded.Notes != "notes" {
		t.Fatalf("task fields not persisted: %+v", loaded)
	}
	if loaded.DueAt == nil || !loaded.DueAt.Equal(due.Truncate(time.Second)) {
		if loaded.DueAt == nil || loaded.DueAt.Unix() != due.Unix() {
			t.Fatalf("due_at not persisted: want %v got %v", due, loaded.DueAt)
		}
	}
	if loaded.Priority == nil || *loaded.Priority != priority {
		t.Fatalf("priority not persisted: %+v", loaded.Priority)
	}
	if loaded.EstimatedMinutes == nil || *loaded.EstimatedMinutes != estimate {
		t.Fatalf("estimate not persisted: %+v", loaded.EstimatedMinutes)
	}
	if loaded.Progress == nil || *loaded.Progress != progress {
		t.Fatalf("progress not persisted: %+v", loaded.Progress)
	}
}

func TestMetaJSONRoundTripPreservesUnknownKeys(t *testing.T) {
	_, taskRepo, _ := setupInMemoryDB(t)
	task := createTask(t, taskRepo, "meta", domain.StatusCreated, nil)
	task.Meta = map[string]any{"known": "before", "future": map[string]any{"a": 1, "b": "x"}}
	base := task.UpdatedAt
	task.UpdatedAt = time.Now().UTC()
	updated, err := taskRepo.UpdateTask(context.Background(), base, task)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	updated.Meta["known"] = "after"
	base = updated.UpdatedAt
	updated.UpdatedAt = time.Now().UTC().Add(time.Second)
	updatedAgain, err := taskRepo.UpdateTask(context.Background(), base, updated)
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if nested, ok := updatedAgain.Meta["future"].(map[string]any); !ok || nested["a"] != float64(1) || nested["b"] != "x" {
		t.Fatalf("unknown meta keys not preserved: %+v", updatedAgain.Meta)
	}
}

func TestSubtasksOrderingAndCascadeDelete(t *testing.T) {
	_, taskRepo, subtaskRepo := setupInMemoryDB(t)
	task := createTask(t, taskRepo, "parent", domain.StatusCreated, nil)
	_, err := subtaskRepo.CreateSubtask(context.Background(), task.ID, "s2", 1)
	if err != nil {
		t.Fatalf("create subtask #1: %v", err)
	}
	_, err = subtaskRepo.CreateSubtask(context.Background(), task.ID, "s1", 0)
	if err != nil {
		t.Fatalf("create subtask #2: %v", err)
	}
	subtasks, err := subtaskRepo.ListSubtasks(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("list subtasks: %v", err)
	}
	if len(subtasks) != 2 || subtasks[0].Title != "s1" || subtasks[1].Title != "s2" {
		t.Fatalf("unexpected subtask order: %+v", subtasks)
	}
	if err := taskRepo.DeleteTask(context.Background(), task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	subtasks, err = subtaskRepo.ListSubtasks(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("list subtasks after delete: %v", err)
	}
	if len(subtasks) != 0 {
		t.Fatalf("expected subtasks to cascade delete, got %d", len(subtasks))
	}
}

func TestOptimisticConcurrencyTaskAndSubtask(t *testing.T) {
	_, taskRepo, subtaskRepo := setupInMemoryDB(t)
	task := createTask(t, taskRepo, "task", domain.StatusCreated, nil)
	staleBase := task.UpdatedAt

	task.Title = "fresh"
	task.UpdatedAt = time.Now().UTC().Add(time.Second)
	updated, err := taskRepo.UpdateTask(context.Background(), staleBase, task)
	if err != nil {
		t.Fatalf("update task with base version: %v", err)
	}
	updated.Title = "stale write"
	updated.UpdatedAt = time.Now().UTC().Add(2 * time.Second)
	if _, err := taskRepo.UpdateTask(context.Background(), staleBase, updated); err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	subtask, err := subtaskRepo.CreateSubtask(context.Background(), task.ID, "sub", 0)
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	staleSubtaskBase := subtask.UpdatedAt
	subtask.Title = "fresh sub"
	subtask.UpdatedAt = time.Now().UTC().Add(time.Second)
	subtask, err = subtaskRepo.UpdateSubtask(context.Background(), staleSubtaskBase, subtask)
	if err != nil {
		t.Fatalf("update subtask: %v", err)
	}
	subtask.Title = "stale sub"
	subtask.UpdatedAt = time.Now().UTC().Add(2 * time.Second)
	if _, err := subtaskRepo.UpdateSubtask(context.Background(), staleSubtaskBase, subtask); err != ErrConflict {
		t.Fatalf("expected subtask ErrConflict, got %v", err)
	}
}

func TestMigrationCreatesBackupFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	databasePath := filepath.Join(home, ".config", "bucket", "bucket.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		t.Fatalf("create db dir: %v", err)
	}
	database, err := Open(databasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	backupsDir := filepath.Join(home, ".config", "bucket", "backups")
	if err := RunMigrations(context.Background(), database, databasePath, backupsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "bucket.db.bak-") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected migration backup to exist")
	}
}

func TestMigrationFailureRestoresBackup(t *testing.T) {
	home := t.TempDir()
	databasePath := filepath.Join(home, "bucket.db")
	database, err := Open(databasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if _, err := database.Exec("CREATE TABLE seed (id INTEGER PRIMARY KEY, v TEXT);"); err != nil {
		t.Fatalf("create seed table: %v", err)
	}
	if _, err := database.Exec("INSERT INTO seed(v) VALUES('before');"); err != nil {
		t.Fatalf("insert seed row: %v", err)
	}
	if _, err := database.Exec("PRAGMA user_version = 0;"); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		t.Fatalf("checkpoint wal: %v", err)
	}

	brokenFS := fstest.MapFS{
		"migrations/001_init.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);")},
		"migrations/002_broken.sql": &fstest.MapFile{Data: []byte("THIS IS INVALID SQL;")},
	}
	backupsDir := filepath.Join(home, "backups")
	err = runMigrationsWithFS(context.Background(), database, databasePath, backupsDir, brokenFS, time.Now)
	if err == nil {
		t.Fatalf("expected migration failure")
	}

	restoredDB, err := Open(databasePath)
	if err != nil {
		t.Fatalf("re-open restored db: %v", err)
	}
	defer restoredDB.Close()
	var value string
	if err := restoredDB.QueryRow("SELECT v FROM seed LIMIT 1").Scan(&value); err != nil {
		t.Fatalf("query restored seed row: %v", err)
	}
	if value != "before" {
		t.Fatalf("expected restored seed value 'before', got %q", value)
	}
}

func TestRefuseNewerUserVersion(t *testing.T) {
	home := t.TempDir()
	databasePath := filepath.Join(home, "bucket.db")
	database, err := Open(databasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec("PRAGMA user_version = 9999;"); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := database.Exec("CREATE TABLE marker (id INTEGER PRIMARY KEY, v TEXT);"); err != nil {
		t.Fatalf("create marker table: %v", err)
	}
	if _, err := database.Exec("INSERT INTO marker(v) VALUES('before');"); err != nil {
		t.Fatalf("insert marker row: %v", err)
	}

	backupsDir := filepath.Join(home, "backups")
	err = RunMigrations(context.Background(), database, databasePath, backupsDir)
	if err == nil {
		t.Fatalf("expected ErrDatabaseTooNew")
	}
	var tooNew ErrDatabaseTooNew
	if !errors.As(err, &tooNew) {
		t.Fatalf("expected ErrDatabaseTooNew, got %v", err)
	}

	var count int
	if scanErr := database.QueryRow("SELECT COUNT(*) FROM marker").Scan(&count); scanErr != nil {
		t.Fatalf("query marker row count: %v", scanErr)
	}
	if count != 1 {
		t.Fatalf("expected marker row unchanged, got %d", count)
	}
}

func TestListQueries(t *testing.T) {
	_, taskRepo, _ := setupInMemoryDB(t)
	loc := time.Local
	now := time.Date(2026, 3, 4, 10, 0, 0, 0, loc)
	startTodayUTC, startTomorrowUTC := domain.LocalDayBoundaries(now)
	yesterdayUTC := startTodayUTC.Add(-24 * time.Hour)

	overdue := startTodayUTC.Add(-2 * time.Hour)
	dueToday := startTodayUTC.Add(10 * time.Hour)
	dueTomorrow := startTomorrowUTC.Add(2 * time.Hour)

	incompleteNoDue := createTask(t, taskRepo, "incomplete-no-due", domain.StatusCreated, nil)
	setTaskCreatedAt(t, taskRepo, incompleteNoDue.ID, yesterdayUTC)
	incompleteOverdue := createTask(t, taskRepo, "incomplete-overdue", domain.StatusInProgress, &overdue)
	setTaskCreatedAt(t, taskRepo, incompleteOverdue.ID, yesterdayUTC)
	completedDueToday := createTask(t, taskRepo, "completed-due-today", domain.StatusCompleted, &dueToday)
	setTaskCreatedAt(t, taskRepo, completedDueToday.ID, yesterdayUTC)
	closedDueToday := createTask(t, taskRepo, "closed-due-today", domain.StatusClosed, &dueToday)
	setTaskCreatedAt(t, taskRepo, closedDueToday.ID, yesterdayUTC)
	archivedToday := createTask(t, taskRepo, "archived-today", domain.StatusArchived, &dueToday)
	setTaskCreatedAt(t, taskRepo, archivedToday.ID, yesterdayUTC)
	futureClosed := createTask(t, taskRepo, "future-closed", domain.StatusClosed, &dueTomorrow)
	setTaskCreatedAt(t, taskRepo, futureClosed.ID, yesterdayUTC)
	futureCreated := createTask(t, taskRepo, "future-created", domain.StatusCreated, &dueTomorrow)
	setTaskCreatedAt(t, taskRepo, futureCreated.ID, yesterdayUTC)
	createdTodayCompleted := createTask(t, taskRepo, "created-today-completed", domain.StatusCompleted, nil)
	setTaskCreatedAt(t, taskRepo, createdTodayCompleted.ID, startTodayUTC.Add(time.Hour))
	completedOldNotDue := createTask(t, taskRepo, "completed-old-not-due", domain.StatusCompleted, nil)
	setTaskCreatedAt(t, taskRepo, completedOldNotDue.ID, yesterdayUTC)

	inbox, err := taskRepo.ListTasks(context.Background(), ListQuery{
		ListType:         domain.ListInbox,
		StartTodayUTC:    startTodayUTC.Unix(),
		StartTomorrowUTC: startTomorrowUTC.Unix(),
	})
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}
	assertContainsTitles(t, inbox, []string{
		"incomplete-no-due",
		"incomplete-overdue",
		"completed-due-today",
		"closed-due-today",
		"future-created",
		"created-today-completed",
	})
	assertNotContainsTitle(t, inbox, "archived-today")
	assertNotContainsTitle(t, inbox, "completed-old-not-due")

	upcoming, err := taskRepo.ListTasks(context.Background(), ListQuery{
		ListType:         domain.ListUpcoming,
		StartTodayUTC:    startTodayUTC.Unix(),
		StartTomorrowUTC: startTomorrowUTC.Unix(),
	})
	if err != nil {
		t.Fatalf("list upcoming: %v", err)
	}
	assertContainsTitles(t, upcoming, []string{"future-closed", "future-created"})
	assertNotContainsTitle(t, upcoming, "incomplete-no-due")

	closed, err := taskRepo.ListTasks(context.Background(), ListQuery{ListType: domain.ListClosed})
	if err != nil {
		t.Fatalf("list closed: %v", err)
	}
	for _, item := range closed {
		if item.Status != domain.StatusClosed && item.Status != domain.StatusCompleted {
			t.Fatalf("unexpected status in closed list: %s", item.Status)
		}
	}

	archived, err := taskRepo.ListTasks(context.Background(), ListQuery{ListType: domain.ListArchived})
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if len(archived) != 1 || archived[0].Status != domain.StatusArchived {
		t.Fatalf("unexpected archived results: %+v", archived)
	}
}

func assertContainsTitles(t *testing.T, items []domain.TaskListItem, want []string) {
	t.Helper()
	titles := map[string]bool{}
	for _, item := range items {
		titles[item.Title] = true
	}
	for _, title := range want {
		if !titles[title] {
			t.Fatalf("expected title %q in list, got %+v", title, titles)
		}
	}
}

func assertNotContainsTitle(t *testing.T, items []domain.TaskListItem, title string) {
	t.Helper()
	for _, item := range items {
		if item.Title == title {
			t.Fatalf("did not expect title %q in list", title)
		}
	}
}

func TestBusyTimeoutReturnsBoundedError(t *testing.T) {
	dir := t.TempDir()
	databasePath := filepath.Join(dir, "bucket.db")
	db1, err := Open(databasePath)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	defer db1.Close()
	if err := RunMigrations(context.Background(), db1, databasePath, filepath.Join(dir, "backups")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	repo1 := NewTaskRepo(db1)
	task := createTask(t, repo1, "locked", domain.StatusCreated, nil)

	db2, err := Open(databasePath)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()

	tx, err := db2.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx db2: %v", err)
	}
	if _, err := tx.Exec("UPDATE tasks SET title = title WHERE id = ?", task.ID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("lock task row: %v", err)
	}

	task.Title = "updated while locked"
	base := task.UpdatedAt
	task.UpdatedAt = time.Now().UTC().Add(time.Second)
	start := time.Now()
	_, err = repo1.UpdateTask(context.Background(), base, task)
	elapsed := time.Since(start)
	_ = tx.Rollback()
	if err == nil {
		t.Fatalf("expected busy/lock error")
	}
	if elapsed > 7*time.Second {
		t.Fatalf("expected bounded-time busy failure, took %s", elapsed)
	}
}

func TestRunMigrationsWithCustomFS(t *testing.T) {
	custom := fstest.MapFS{
		"migrations/001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);")},
	}
	migrations, latest, err := loadMigrations(custom)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if latest != 1 || len(migrations) != 1 {
		t.Fatalf("unexpected migrations parsed: latest=%d len=%d", latest, len(migrations))
	}
}

func TestLoadMigrationsRejectsBadName(t *testing.T) {
	custom := fstest.MapFS{
		"migrations/bad.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	_, _, err := loadMigrations(custom)
	if err == nil {
		t.Fatalf("expected bad migration name error")
	}
}

func TestMigrationSourceImplementsFS(t *testing.T) {
	var _ fs.FS = fstest.MapFS{}
}
