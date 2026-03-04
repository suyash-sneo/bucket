package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"bucket/internal/domain"
)

type SQLiteSubtaskRepo struct {
	db *sql.DB
}

func NewSubtaskRepo(db *sql.DB) *SQLiteSubtaskRepo {
	return &SQLiteSubtaskRepo{db: db}
}

func (repository *SQLiteSubtaskRepo) ListSubtasks(ctx context.Context, taskID int64) ([]domain.Subtask, error) {
	rows, err := repository.db.QueryContext(ctx, `SELECT id, task_id, title, status, position, meta_json, created_at, updated_at
		FROM subtasks WHERE task_id=? ORDER BY position ASC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list subtasks for task %d: %w", taskID, err)
	}
	defer rows.Close()

	subtasks := make([]domain.Subtask, 0, 8)
	for rows.Next() {
		var (
			subtask     domain.Subtask
			metaJSON    string
			createdUnix int64
			updatedUnix int64
		)
		if err := rows.Scan(&subtask.ID, &subtask.TaskID, &subtask.Title, &subtask.Status, &subtask.Position, &metaJSON, &createdUnix, &updatedUnix); err != nil {
			return nil, fmt.Errorf("scan subtask row: %w", err)
		}
		meta, err := domain.ParseMetaJSON(metaJSON)
		if err != nil {
			return nil, fmt.Errorf("decode subtask meta: %w", err)
		}
		subtask.Meta = meta
		subtask.CreatedAt = time.Unix(createdUnix, 0).UTC()
		subtask.UpdatedAt = time.Unix(updatedUnix, 0).UTC()
		subtasks = append(subtasks, subtask)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtask rows: %w", err)
	}
	return subtasks, nil
}

func (repository *SQLiteSubtaskRepo) CreateSubtask(ctx context.Context, taskID int64, title string, position int) (domain.Subtask, error) {
	now := time.Now().UTC()
	subtask := domain.Subtask{
		TaskID:    taskID,
		Title:     title,
		Status:    domain.StatusCreated,
		Position:  position,
		Meta:      map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := domain.ValidateSubtask(&subtask); err != nil {
		return domain.Subtask{}, fmt.Errorf("validate subtask: %w", err)
	}

	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("begin create subtask tx: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `INSERT INTO subtasks (task_id, title, status, position, meta_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		subtask.TaskID,
		subtask.Title,
		subtask.Status,
		subtask.Position,
		domain.MustMetaJSON(subtask.Meta),
		subtask.CreatedAt.Unix(),
		subtask.UpdatedAt.Unix(),
	)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("insert subtask: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("fetch subtask id: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return domain.Subtask{}, fmt.Errorf("commit create subtask tx: %w", err)
	}
	subtask.ID = id
	return subtask, nil
}

func (repository *SQLiteSubtaskRepo) UpdateSubtask(ctx context.Context, baseUpdatedAt time.Time, subtask domain.Subtask) (domain.Subtask, error) {
	if err := domain.ValidateSubtask(&subtask); err != nil {
		return domain.Subtask{}, fmt.Errorf("validate subtask: %w", err)
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("begin update subtask tx: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `UPDATE subtasks SET title=?, status=?, position=?, meta_json=?, updated_at=?
		WHERE id=? AND updated_at=?`,
		subtask.Title,
		subtask.Status,
		subtask.Position,
		domain.MustMetaJSON(subtask.Meta),
		subtask.UpdatedAt.UTC().Unix(),
		subtask.ID,
		baseUpdatedAt.UTC().Unix(),
	)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("update subtask %d: %w", subtask.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("check updated subtask rows affected: %w", err)
	}
	if affected == 0 {
		return domain.Subtask{}, ErrConflict
	}
	if err := transaction.Commit(); err != nil {
		return domain.Subtask{}, fmt.Errorf("commit update subtask tx: %w", err)
	}
	fresh, err := repository.getSubtaskByID(ctx, subtask.ID)
	if err != nil {
		return domain.Subtask{}, err
	}
	return fresh, nil
}

func (repository *SQLiteSubtaskRepo) DeleteSubtask(ctx context.Context, id int64) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete subtask tx: %w", err)
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, "DELETE FROM subtasks WHERE id=?", id); err != nil {
		return fmt.Errorf("delete subtask %d: %w", id, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit delete subtask tx: %w", err)
	}
	return nil
}

func (repository *SQLiteSubtaskRepo) ReorderSubtask(ctx context.Context, id int64, newPosition int) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder subtask tx: %w", err)
	}
	defer transaction.Rollback()

	var taskID int64
	var oldPosition int
	if err := transaction.QueryRowContext(ctx, "SELECT task_id, position FROM subtasks WHERE id=?", id).Scan(&taskID, &oldPosition); err != nil {
		return fmt.Errorf("load subtask for reorder: %w", err)
	}
	if oldPosition == newPosition {
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit no-op reorder tx: %w", err)
		}
		return nil
	}
	now := time.Now().UTC().Unix()
	var swappedID sql.NullInt64
	if err := transaction.QueryRowContext(ctx, "SELECT id FROM subtasks WHERE task_id=? AND position=?", taskID, newPosition).Scan(&swappedID); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("find subtask to swap positions: %w", err)
	}
	if swappedID.Valid {
		if _, err := transaction.ExecContext(ctx, "UPDATE subtasks SET position=?, updated_at=? WHERE id=?", oldPosition, now, swappedID.Int64); err != nil {
			return fmt.Errorf("update swapped subtask position: %w", err)
		}
	}
	if _, err := transaction.ExecContext(ctx, "UPDATE subtasks SET position=?, updated_at=? WHERE id=?", newPosition, now, id); err != nil {
		return fmt.Errorf("update reordered subtask position: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit reorder subtask tx: %w", err)
	}
	return nil
}

func (repository *SQLiteSubtaskRepo) getSubtaskByID(ctx context.Context, id int64) (domain.Subtask, error) {
	row := repository.db.QueryRowContext(ctx, `SELECT id, task_id, title, status, position, meta_json, created_at, updated_at
		FROM subtasks WHERE id=?`, id)
	var (
		subtask     domain.Subtask
		metaJSON    string
		createdUnix int64
		updatedUnix int64
	)
	if err := row.Scan(&subtask.ID, &subtask.TaskID, &subtask.Title, &subtask.Status, &subtask.Position, &metaJSON, &createdUnix, &updatedUnix); err != nil {
		return domain.Subtask{}, fmt.Errorf("get subtask %d: %w", id, err)
	}
	meta, err := domain.ParseMetaJSON(metaJSON)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("decode subtask meta: %w", err)
	}
	subtask.Meta = meta
	subtask.CreatedAt = time.Unix(createdUnix, 0).UTC()
	subtask.UpdatedAt = time.Unix(updatedUnix, 0).UTC()
	return subtask, nil
}
