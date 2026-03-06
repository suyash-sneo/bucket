package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Migration struct {
	Version int
	Name    string
	SQL     string
}

type ErrDatabaseTooNew struct {
	UserVersion int
	LatestKnown int
}

func (err ErrDatabaseTooNew) Error() string {
	return fmt.Sprintf("database user_version %d is newer than supported version %d", err.UserVersion, err.LatestKnown)
}

func LatestKnownVersion() int {
	migrations, _, loadErr := loadMigrations(migrationFS)
	if loadErr != nil || len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].Version
}

func RunMigrations(ctx context.Context, database *sql.DB, databasePath, backupsDir string) error {
	return runMigrationsWithFS(ctx, database, databasePath, backupsDir, migrationFS, time.Now)
}

func runMigrationsWithFS(ctx context.Context, database *sql.DB, databasePath, backupsDir string, source fs.FS, now func() time.Time) error {
	migrations, latestKnownVersion, err := loadMigrations(source)
	if err != nil {
		return err
	}

	userVersion, err := UserVersion(ctx, database)
	if err != nil {
		return err
	}
	if userVersion > latestKnownVersion {
		return ErrDatabaseTooNew{UserVersion: userVersion, LatestKnown: latestKnownVersion}
	}

	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	);`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	appliedVersions, err := loadAppliedVersions(ctx, database)
	if err != nil {
		return err
	}

	pending := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if _, exists := appliedVersions[migration.Version]; !exists {
			pending = append(pending, migration)
		}
	}

	backupPath := ""
	if len(pending) > 0 && isFileBackedDB(databasePath) {
		backupPath, err = createMigrationBackup(ctx, database, databasePath, backupsDir, now)
		if err != nil {
			return err
		}
	}

	for _, migration := range pending {
		if err := applyMigration(ctx, database, migration, now); err != nil {
			if backupPath != "" {
				if restoreErr := restoreBackup(database, databasePath, backupPath); restoreErr != nil {
					return fmt.Errorf("migration %03d failed: %v (restore backup failed: %v)", migration.Version, err, restoreErr)
				}
			}
			return fmt.Errorf("migration %03d failed: %w", migration.Version, err)
		}
	}

	if _, err := database.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", latestKnownVersion)); err != nil {
		if backupPath != "" {
			_ = restoreBackup(database, databasePath, backupPath)
		}
		return fmt.Errorf("set user_version=%d: %w", latestKnownVersion, err)
	}

	if err := QuickCheck(ctx, database); err != nil {
		if backupPath != "" {
			if restoreErr := restoreBackup(database, databasePath, backupPath); restoreErr != nil {
				return fmt.Errorf("quick_check after migrations failed: %v (restore backup failed: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("quick_check after migrations failed: %w", err)
	}

	return nil
}

func loadMigrations(source fs.FS) ([]Migration, int, error) {
	entries, err := fs.ReadDir(source, "migrations")
	if err != nil {
		return nil, 0, fmt.Errorf("read migrations directory: %w", err)
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return nil, 0, err
		}
		payload, err := fs.ReadFile(source, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, 0, fmt.Errorf("read migration file %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    entry.Name(),
			SQL:     string(payload),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	latest := 0
	if len(migrations) > 0 {
		latest = migrations[len(migrations)-1].Version
	}
	return migrations, latest, nil
}

func parseMigrationVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid migration filename %s", name)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %s: %w", name, err)
	}
	return version, nil
}

func loadAppliedVersions(ctx context.Context, database *sql.DB) (map[int]struct{}, error) {
	rows, err := database.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]struct{})
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan schema_migrations row: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations rows: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, database *sql.DB, migration Migration, now func() time.Time) error {
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire db connection for migration %03d: %w", migration.Version, err)
	}
	defer connection.Close()

	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE;"); err != nil {
		return fmt.Errorf("begin immediate tx: %w", err)
	}
	rollback := func() {
		_, _ = connection.ExecContext(context.Background(), "ROLLBACK;")
	}

	if _, err := connection.ExecContext(ctx, migration.SQL); err != nil {
		rollback()
		return fmt.Errorf("execute migration SQL: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)", migration.Version, now().UTC().Unix()); err != nil {
		rollback()
		return fmt.Errorf("insert schema_migrations row: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "COMMIT;"); err != nil {
		rollback()
		return fmt.Errorf("commit migration tx: %w", err)
	}
	return nil
}

func createMigrationBackup(ctx context.Context, database *sql.DB, databasePath, backupsDir string, now func() time.Time) (string, error) {
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		return "", fmt.Errorf("create backups dir %s: %w", backupsDir, err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return "", fmt.Errorf("wal checkpoint before backup: %w", err)
	}

	source, err := os.Open(databasePath)
	if err != nil {
		return "", fmt.Errorf("open db for backup: %w", err)
	}
	defer source.Close()

	filename := fmt.Sprintf("%s.bak-%s", filepath.Base(databasePath), now().Format("20060102-150405"))
	backupPath := filepath.Join(backupsDir, filename)
	target, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create backup file %s: %w", backupPath, err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		return "", fmt.Errorf("copy database to backup %s: %w", backupPath, err)
	}
	if err := target.Close(); err != nil {
		return "", fmt.Errorf("close backup file %s: %w", backupPath, err)
	}
	if err := pruneOldBackups(backupsDir, filepath.Base(databasePath), 20); err != nil {
		return "", err
	}
	return backupPath, nil
}

func pruneOldBackups(backupsDir, dbBase string, maxKeep int) error {
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return fmt.Errorf("read backups dir %s: %w", backupsDir, err)
	}
	prefix := dbBase + ".bak-"
	backups := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) {
			backups = append(backups, entry.Name())
		}
	}
	sort.Strings(backups)
	if len(backups) <= maxKeep {
		return nil
	}
	for _, name := range backups[:len(backups)-maxKeep] {
		if err := os.Remove(filepath.Join(backupsDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove old backup %s: %w", name, err)
		}
	}
	return nil
}

func restoreBackup(database *sql.DB, databasePath, backupPath string) error {
	_ = database.Close()
	if backupPath == "" {
		return nil
	}
	source, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup %s: %w", backupPath, err)
	}
	defer source.Close()

	_ = os.Remove(databasePath + "-wal")
	_ = os.Remove(databasePath + "-shm")

	target, err := os.OpenFile(databasePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open db file for restore %s: %w", databasePath, err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		return fmt.Errorf("restore backup to db file %s: %w", databasePath, err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close restored db file %s: %w", databasePath, err)
	}
	return nil
}

func isFileBackedDB(path string) bool {
	if path == "" {
		return false
	}
	lower := strings.ToLower(path)
	if strings.Contains(lower, ":memory:") {
		return false
	}
	if strings.HasPrefix(lower, "file::memory:") {
		return false
	}
	return true
}
