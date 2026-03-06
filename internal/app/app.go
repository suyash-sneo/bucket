package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"bucket/internal/config"
	"bucket/internal/db"
	"bucket/internal/logging"
	"bucket/internal/service"
	"bucket/internal/ui"
	uitheme "bucket/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

const DefaultVersion = "dev"

type Runtime struct {
	Version string
	Logger  *slog.Logger
	DB      *db.SQLiteTaskRepo
}

func Run(version string) error {
	if version == "" {
		version = DefaultVersion
	}
	configDir, err := config.EnsureConfigDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(configDir, "config.yml")
	logPath := filepath.Join(configDir, "log.txt")

	logSetup, err := logging.Setup(logPath, 10)
	if err != nil {
		return fmt.Errorf("initialize logging: %w", err)
	}
	defer logSetup.Writer.Close()
	logger := logSetup.Logger

	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		logger.Error("config error", "error", err)
		return fmt.Errorf("failed to load config %s: %w", configPath, err)
	}
	if cfg.LogMaxMB != 10 {
		_ = logSetup.Writer.Close()
		logSetup, err = logging.Setup(logPath, cfg.LogMaxMB)
		if err != nil {
			return fmt.Errorf("reconfigure logging: %w", err)
		}
		defer logSetup.Writer.Close()
		logger = logSetup.Logger
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	logger.Info("startup", logging.StartupFields(version, configPath, cfg.DatabasePath)...)

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("database open failed", "error", err)
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := db.QuickCheck(ctx, database); err != nil {
		logger.Error("quick_check before migrations failed", "error", err)
		return err
	}

	backupsDir := filepath.Join(configDir, "backups")
	if err := db.RunMigrations(ctx, database, cfg.DatabasePath, backupsDir); err != nil {
		logger.Error("migration failed", "error", err)
		return err
	}
	if err := db.QuickCheck(ctx, database); err != nil {
		logger.Error("quick_check after migrations failed", "error", err)
		return err
	}

	taskRepo := db.NewTaskRepo(database)
	subtaskRepo := db.NewSubtaskRepo(database)
	taskService := service.NewTaskService(taskRepo, subtaskRepo, time.Now)

	draftsDir := filepath.Join(configDir, "drafts")
	if err := os.MkdirAll(draftsDir, 0o700); err != nil {
		logger.Error("create drafts directory failed", "error", err)
		return err
	}
	conflicts, err := recoverDrafts(taskService, draftsDir, logger)
	if err != nil {
		logger.Error("draft recovery failed", "error", err)
		return err
	}

	model := ui.NewModel(ui.ModelOptions{
		Service:        taskService,
		Theme:          uitheme.Resolve(cfg.Theme),
		ListType:       "inbox",
		DraftsDir:      draftsDir,
		ConflictDrafts: conflicts,
		Now:            time.Now,
		Logger:         logger,
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		logger.Error("ui program failed", "error", err)
		return fmt.Errorf("run UI: %w", err)
	}
	return nil
}

func recoverDrafts(taskService *service.TaskService, draftsDir string, logger *slog.Logger) ([]ui.DraftConflict, error) {
	drafts, _, err := ui.LoadDraftFiles(draftsDir)
	if err != nil {
		return nil, err
	}
	conflicts := make([]ui.DraftConflict, 0)
	for _, draft := range drafts {
		task, _, err := taskService.GetDetails(draft.TaskID)
		path := ui.DraftFilePath(draftsDir, draft.TaskID)
		if err != nil {
			conflicts = append(conflicts, ui.DraftConflict{TaskID: draft.TaskID, Path: path, Draft: draft})
			continue
		}
		if task.UpdatedAt.Unix() != draft.BaseUpdatedAt {
			conflicts = append(conflicts, ui.DraftConflict{TaskID: draft.TaskID, Path: path, Draft: draft})
			continue
		}
		patched, err := ui.ApplyDraftToTask(task, draft)
		if err != nil {
			logger.Error("failed to apply draft", "task_id", draft.TaskID, "error", err)
			conflicts = append(conflicts, ui.DraftConflict{TaskID: draft.TaskID, Path: path, Draft: draft})
			continue
		}
		if _, err := taskService.UpdateTask(task.UpdatedAt, patched); err != nil {
			logger.Error("failed to persist recovered draft", "task_id", draft.TaskID, "error", err)
			conflicts = append(conflicts, ui.DraftConflict{TaskID: draft.TaskID, Path: path, Draft: draft})
			continue
		}
		if err := ui.DeleteDraftFile(draftsDir, draft.TaskID); err != nil {
			logger.Error("failed to delete recovered draft", "task_id", draft.TaskID, "error", err)
		}
	}
	return conflicts, nil
}

func PanicGuard(logPath string, fn func() error) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		stack := string(debug.Stack())
		message := fmt.Sprintf("panic: %v\n%s", recovered, stack)
		_ = os.WriteFile(logPath, []byte(message+"\n"), 0o600)
		fmt.Fprintf(os.Stderr, "buckets crashed. See %s\n", logPath)
		err = errors.New("panic recovered")
	}()
	return fn()
}

func UserFacingConfigError(path string, err error) string {
	return strings.TrimSpace(fmt.Sprintf("Failed to load config:\n%s\n\n%s", path, err.Error()))
}
