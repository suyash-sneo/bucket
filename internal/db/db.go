package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s: %w", path, err)
	}
	database.SetMaxOpenConns(1)
	database.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA foreign_keys=ON;",
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA temp_store=MEMORY;",
	}
	for _, statement := range pragmas {
		if _, err := database.Exec(statement); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", statement, err)
		}
	}
	return database, nil
}

func QuickCheck(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, "PRAGMA quick_check;")
	if err != nil {
		return fmt.Errorf("run quick_check: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return fmt.Errorf("scan quick_check result: %w", err)
		}
		if value != "ok" {
			return fmt.Errorf("quick_check failed: %s", value)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate quick_check rows: %w", err)
	}
	return nil
}

func UserVersion(ctx context.Context, database *sql.DB) (int, error) {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&version); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}
	return version, nil
}
