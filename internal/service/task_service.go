package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bucket/internal/db"
	"bucket/internal/domain"
)

type TaskRepo interface {
	CreateTask(ctx context.Context, task domain.Task) (domain.Task, error)
	UpdateTask(ctx context.Context, baseUpdatedAt time.Time, task domain.Task) (domain.Task, error)
	GetTask(ctx context.Context, id int64) (domain.Task, []domain.Subtask, error)
	DeleteTask(ctx context.Context, id int64) error
	ListTasks(ctx context.Context, query db.ListQuery) ([]domain.TaskListItem, error)
}

type SubtaskRepo interface {
	ListSubtasks(ctx context.Context, taskID int64) ([]domain.Subtask, error)
	CreateSubtask(ctx context.Context, taskID int64, title string, position int) (domain.Subtask, error)
	UpdateSubtask(ctx context.Context, baseUpdatedAt time.Time, subtask domain.Subtask) (domain.Subtask, error)
	DeleteSubtask(ctx context.Context, id int64) error
	ReorderSubtask(ctx context.Context, id int64, newPosition int) error
}

type ConflictError struct {
	LatestTask     domain.Task
	LatestSubtasks []domain.Subtask
	Cause          error
}

func (err *ConflictError) Error() string {
	if err.Cause == nil {
		return "conflict"
	}
	return fmt.Sprintf("conflict: %v", err.Cause)
}

func (err *ConflictError) Unwrap() error {
	return err.Cause
}

func IsConflict(err error) bool {
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		return true
	}
	return errors.Is(err, db.ErrConflict)
}

type TaskService struct {
	tasks    TaskRepo
	subtasks SubtaskRepo
	now      func() time.Time
}

func NewTaskService(taskRepo TaskRepo, subtaskRepo SubtaskRepo, now func() time.Time) *TaskService {
	if now == nil {
		now = time.Now
	}
	return &TaskService{tasks: taskRepo, subtasks: subtaskRepo, now: now}
}

func (service *TaskService) QuickAdd(title string) (domain.Task, error) {
	now := service.now().UTC()
	task := domain.Task{
		Title:     title,
		Status:    domain.StatusCreated,
		Meta:      map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := domain.ValidateTask(&task); err != nil {
		return domain.Task{}, fmt.Errorf("quick add task: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	created, err := service.tasks.CreateTask(ctx, task)
	if err != nil {
		return domain.Task{}, fmt.Errorf("quick add task: %w", err)
	}
	return created, nil
}

func (service *TaskService) CycleTaskStatus(id int64, baseUpdatedAt time.Time) (domain.Task, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	current, subtasks, err := service.tasks.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("load task %d for status cycle: %w", id, err)
	}
	current.Status = domain.CycleStatus(current.Status)
	current.UpdatedAt = service.now().UTC()
	updated, err := service.tasks.UpdateTask(ctx, baseUpdatedAt, current)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			latest, latestSubtasks, getErr := service.tasks.GetTask(ctx, id)
			if getErr != nil {
				return domain.Task{}, fmt.Errorf("task status conflict and reload failed: %w", getErr)
			}
			return latest, &ConflictError{LatestTask: latest, LatestSubtasks: latestSubtasks, Cause: err}
		}
		return domain.Task{}, fmt.Errorf("cycle task status for task %d: %w", id, err)
	}
	if len(subtasks) > 0 {
		_ = subtasks
	}
	return updated, nil
}

func (service *TaskService) UpdateTask(baseUpdatedAt time.Time, task domain.Task) (domain.Task, error) {
	task.UpdatedAt = service.now().UTC()
	if err := domain.ValidateTask(&task); err != nil {
		return domain.Task{}, fmt.Errorf("update task %d validation failed: %w", task.ID, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, err := service.tasks.UpdateTask(ctx, baseUpdatedAt, task)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			latest, latestSubtasks, getErr := service.tasks.GetTask(ctx, task.ID)
			if getErr != nil {
				return domain.Task{}, fmt.Errorf("task update conflict and reload failed: %w", getErr)
			}
			return latest, &ConflictError{LatestTask: latest, LatestSubtasks: latestSubtasks, Cause: err}
		}
		return domain.Task{}, fmt.Errorf("update task %d: %w", task.ID, err)
	}
	return updated, nil
}

func (service *TaskService) GetDetails(id int64) (domain.Task, []domain.Subtask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, subtasks, err := service.tasks.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, nil, fmt.Errorf("load task %d details: %w", id, err)
	}
	return task, subtasks, nil
}

func (service *TaskService) List(listType string, now time.Time) ([]domain.TaskListItem, error) {
	startTodayUTC, startTomorrowUTC := domain.LocalDayBoundaries(now)
	query := db.ListQuery{
		ListType:         listType,
		Sort:             db.SortInboxDefault,
		IncludeArchived:  listType == domain.ListAll,
		StartTodayUTC:    startTodayUTC.Unix(),
		StartTomorrowUTC: startTomorrowUTC.Unix(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := service.tasks.ListTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tasks for %s: %w", listType, err)
	}
	return items, nil
}

func (service *TaskService) CreateSubtask(taskID int64, title string, position int) (domain.Subtask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subtask, err := service.subtasks.CreateSubtask(ctx, taskID, title, position)
	if err != nil {
		return domain.Subtask{}, fmt.Errorf("create subtask: %w", err)
	}
	return subtask, nil
}

func (service *TaskService) UpdateSubtask(baseUpdatedAt time.Time, subtask domain.Subtask) (domain.Subtask, error) {
	subtask.UpdatedAt = service.now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, err := service.subtasks.UpdateSubtask(ctx, baseUpdatedAt, subtask)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			return domain.Subtask{}, &ConflictError{Cause: err}
		}
		return domain.Subtask{}, fmt.Errorf("update subtask %d: %w", subtask.ID, err)
	}
	return updated, nil
}

func (service *TaskService) DeleteSubtask(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.subtasks.DeleteSubtask(ctx, id); err != nil {
		return fmt.Errorf("delete subtask %d: %w", id, err)
	}
	return nil
}

func (service *TaskService) ReorderSubtask(id int64, newPosition int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.subtasks.ReorderSubtask(ctx, id, newPosition); err != nil {
		return fmt.Errorf("reorder subtask %d: %w", id, err)
	}
	return nil
}
