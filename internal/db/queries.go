package db

import (
	"fmt"

	"bucket/internal/domain"
)

type ListSort string

const (
	SortInboxDefault ListSort = "inbox_default"
	SortDueAsc       ListSort = "due_asc"
	SortUpdatedDesc  ListSort = "updated_desc"
	SortCreatedDesc  ListSort = "created_desc"
	SortPriorityDesc ListSort = "priority_desc"
)

type ListQuery struct {
	ListType         string
	Sort             ListSort
	IncludeArchived  bool
	StartTodayUTC    int64
	StartTomorrowUTC int64
}

const taskListSelect = `SELECT id, title, status, due_at, priority, updated_at, created_at
FROM tasks`

func BuildListSQL(query ListQuery) (string, []any, error) {
	where := ""
	args := make([]any, 0, 3)
	orderBy := ""

	switch query.ListType {
	case domain.ListInbox:
		where = "status != 'archived' AND ( (created_at >= ? AND created_at < ?) OR (due_at >= ? AND due_at < ?) OR (status NOT IN ('closed','completed','archived')) )"
		args = append(args, query.StartTodayUTC, query.StartTomorrowUTC, query.StartTodayUTC, query.StartTomorrowUTC)
		orderBy = "CASE WHEN due_at IS NULL THEN 1 ELSE 0 END, due_at ASC, COALESCE(priority,0) DESC, updated_at DESC"
	case domain.ListUpcoming:
		where = "status != 'archived' AND due_at >= ?"
		args = append(args, query.StartTomorrowUTC)
		orderBy = "due_at ASC, COALESCE(priority,0) DESC, updated_at DESC"
	case domain.ListAll:
		if !query.IncludeArchived {
			where = "status != 'archived'"
		}
		orderBy = "updated_at DESC"
	case domain.ListClosed:
		where = "status IN ('closed','completed')"
		orderBy = "updated_at DESC"
	case domain.ListArchived:
		where = "status = 'archived'"
		orderBy = "updated_at DESC"
	default:
		return "", nil, fmt.Errorf("unsupported list type %q", query.ListType)
	}

	if query.Sort != "" {
		orderBy = resolveSort(query.Sort, orderBy)
	}

	sqlStatement := taskListSelect
	if where != "" {
		sqlStatement += "\nWHERE " + where
	}
	sqlStatement += "\nORDER BY " + orderBy
	return sqlStatement, args, nil
}

func resolveSort(sort ListSort, fallback string) string {
	switch sort {
	case SortInboxDefault:
		return fallback
	case SortDueAsc:
		return "CASE WHEN due_at IS NULL THEN 1 ELSE 0 END, due_at ASC, updated_at DESC"
	case SortUpdatedDesc:
		return "updated_at DESC"
	case SortCreatedDesc:
		return "created_at DESC"
	case SortPriorityDesc:
		return "COALESCE(priority,0) DESC, updated_at DESC"
	default:
		return fallback
	}
}
