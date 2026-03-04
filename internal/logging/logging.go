package logging

import (
	"fmt"
	"log/slog"
	"runtime"
)

type SetupResult struct {
	Logger *slog.Logger
	Writer *CappedWriter
}

func Setup(logPath string, maxMB int) (SetupResult, error) {
	maxBytes := int64(maxMB) * 1024 * 1024
	writer, err := NewCappedWriter(logPath, maxBytes)
	if err != nil {
		return SetupResult{}, err
	}
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return SetupResult{Logger: logger, Writer: writer}, nil
}

func StartupFields(version, configPath, dbPath string) []any {
	return []any{
		"version", version,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"config_path", configPath,
		"db_path", dbPath,
	}
}

func WrapError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
