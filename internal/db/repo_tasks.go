package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bucket/internal/domain"
)

var ErrConflict = errors.New("conflict")

type SQLiteTaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *SQLiteTaskRepo {
	return &SQLiteTaskRepo{db: db}
}

func (repository *SQLiteTaskRepo) CreateTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	if err := domain.ValidateTask(&task); err != nil {
		return domain.Task{}, fmt.Errorf("validate task: %w", err)
	}
	nowUnix := task.CreatedAt.UTC().Unix()
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	updatedUnix := task.UpdatedAt.UTC().Unix()

	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin create task tx: %w", err)
	}
	defer transaction.Rollback()

	result, err := transaction.ExecContext(ctx, `INSERT INTO tasks
		(title, status, url, notes, due_at, priority, est_minutes, progress, meta_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title,
		task.Status,
		nullIfEmpty(task.URL),
		nullIfEmpty(task.Notes),
		timePtrToUnix(task.DueAt),
		intPtrToAny(task.Priority),
		intPtrToAny(task.EstimatedMinutes),
		intPtrToAny(task.Progress),
		domain.MustMetaJSON(task.Meta),
		nowUnix,
		updatedUnix,
	)
	if err != nil {
		return domain.Task{}, fmt.Errorf("insert task: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Task{}, fmt.Errorf("fetch inserted task id: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit create task tx: %w", err)
	}
	created, _, err := repository.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	return created, nil
}

func (repository *SQLiteTaskRepo) UpdateTask(ctx context.Context, baseUpdatedAt time.Time, task domain.Task) (domain.Task, error) {
	if err := domain.ValidateTask(&task); err != nil {
		return domain.Task{}, fmt.Errorf("validate task: %w", err)
	}

	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin update task tx: %w", err)
	}
	defer transaction.Rollback()

	result, err := transaction.ExecContext(ctx, `UPDATE tasks SET
		title=?, status=?, url=?, notes=?, due_at=?, priority=?, est_minutes=?, progress=?, meta_json=?, updated_at=?
		WHERE id=? AND updated_at=?`,
		task.Title,
		task.Status,
		nullIfEmpty(task.URL),
		nullIfEmpty(task.Notes),
		timePtrToUnix(task.DueAt),
		intPtrToAny(task.Priority),
		intPtrToAny(task.EstimatedMinutes),
		intPtrToAny(task.Progress),
		domain.MustMetaJSON(task.Meta),
		task.UpdatedAt.UTC().Unix(),
		task.ID,
		baseUpdatedAt.UTC().Unix(),
	)
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task %d: %w", task.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Task{}, fmt.Errorf("check update rows affected: %w", err)
	}
	if affected == 0 {
		return domain.Task{}, ErrConflict
	}
	if err := transaction.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit update task tx: %w", err)
	}
	updated, _, err := repository.GetTask(ctx, task.ID)
	if err != nil {
		return domain.Task{}, err
	}
	return updated, nil
}

func (repository *SQLiteTaskRepo) GetTask(ctx context.Context, id int64) (domain.Task, []domain.Subtask, error) {
	row := repository.db.QueryRowContext(ctx, `SELECT id, title, status, url, notes, due_at, priority, est_minutes, progress, meta_json, created_at, updated_at
		FROM tasks WHERE id=?`, id)
	var (
		task          domain.Task
		urlValue      sql.NullString
		notesValue    sql.NullString
		dueAtUnix     sql.NullInt64
		priorityValue sql.NullInt64
		estimateValue sql.NullInt64
		progressValue sql.NullInt64
		metaJSON      string
		createdUnix   int64
		updatedUnix   int64
	)
	if err := row.Scan(
		&task.ID,
		&task.Title,
		&task.Status,
		&urlValue,
		&notesValue,
		&dueAtUnix,
		&priorityValue,
		&estimateValue,
		&progressValue,
		&metaJSON,
		&createdUnix,
		&updatedUnix,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, nil, fmt.Errorf("task %d not found", id)
		}
		return domain.Task{}, nil, fmt.Errorf("get task %d: %w", id, err)
	}
	if urlValue.Valid {
		task.URL = urlValue.String
	}
	if notesValue.Valid {
		task.Notes = notesValue.String
	}
	task.DueAt = unixToTimePtr(dueAtUnix)
	task.Priority = nullInt64ToIntPtr(priorityValue)
	task.EstimatedMinutes = nullInt64ToIntPtr(estimateValue)
	task.Progress = nullInt64ToIntPtr(progressValue)
	meta, err := domain.ParseMetaJSON(metaJSON)
	if err != nil {
		return domain.Task{}, nil, fmt.Errorf("decode task meta: %w", err)
	}
	task.Meta = meta
	task.CreatedAt = time.Unix(createdUnix, 0).UTC()
	task.UpdatedAt = time.Unix(updatedUnix, 0).UTC()

	subtasks, err := NewSubtaskRepo(repository.db).ListSubtasks(ctx, id)
	if err != nil {
		return domain.Task{}, nil, err
	}
	return task, subtasks, nil
}

func (repository *SQLiteTaskRepo) DeleteTask(ctx context.Context, id int64) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete task tx: %w", err)
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, "DELETE FROM tasks WHERE id=?", id); err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit delete task tx: %w", err)
	}
	return nil
}

func (repository *SQLiteTaskRepo) ListTasks(ctx context.Context, query ListQuery) ([]domain.TaskListItem, error) {
	sqlStatement, args, err := BuildListSQL(query)
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, sqlStatement, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	items := make([]domain.TaskListItem, 0, 32)
	for rows.Next() {
		var (
			item      domain.TaskListItem
			dueAtUnix sql.NullInt64
			priority  sql.NullInt64
			updatedAt int64
			createdAt int64
		)
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &dueAtUnix, &priority, &updatedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan task list row: %w", err)
		}
		item.DueAt = unixToTimePtr(dueAtUnix)
		item.Priority = nullInt64ToIntPtr(priority)
		item.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		item.CreatedAt = time.Unix(createdAt, 0).UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task list rows: %w", err)
	}
	return items, nil
}

func nullIfEmpty(input string) any {
	if input == "" {
		return nil
	}
	return input
}

func timePtrToUnix(input *time.Time) any {
	if input == nil {
		return nil
	}
	return input.UTC().Unix()
}

func unixToTimePtr(input sql.NullInt64) *time.Time {
	if !input.Valid {
		return nil
	}
	value := time.Unix(input.Int64, 0).UTC()
	return &value
}

func intPtrToAny(input *int) any {
	if input == nil {
		return nil
	}
	return *input
}

func nullInt64ToIntPtr(input sql.NullInt64) *int {
	if !input.Valid {
		return nil
	}
	value := int(input.Int64)
	return &value
}
