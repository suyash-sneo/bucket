package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	StatusCreated    = "created"
	StatusInProgress = "in_progress"
	StatusClosed     = "closed"
	StatusCompleted  = "completed"
	StatusArchived   = "archived"
)

const (
	ListInbox    = "inbox"
	ListUpcoming = "upcoming"
	ListAll      = "all"
	ListClosed   = "closed"
	ListArchived = "archived"
)

var statusCycle = []string{
	StatusCreated,
	StatusInProgress,
	StatusCompleted,
	StatusClosed,
	StatusArchived,
}

var knownStatuses = map[string]struct{}{
	StatusCreated:    {},
	StatusInProgress: {},
	StatusClosed:     {},
	StatusCompleted:  {},
	StatusArchived:   {},
}

type Task struct {
	ID               int64
	Title            string
	Status           string
	URL              string
	Notes            string
	DueAt            *time.Time
	Priority         *int
	EstimatedMinutes *int
	Progress         *int
	Meta             map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TaskListItem struct {
	ID        int64
	Title     string
	Status    string
	DueAt     *time.Time
	Priority  *int
	UpdatedAt time.Time
	CreatedAt time.Time
}

type Subtask struct {
	ID        int64
	TaskID    int64
	Title     string
	Status    string
	Position  int
	Meta      map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrInvalidTitle      = errors.New("title is required")
	ErrInvalidPriority   = errors.New("priority must be between 1 and 4")
	ErrInvalidProgress   = errors.New("progress must be between 0 and 100")
	ErrInvalidEstimate   = errors.New("estimated minutes must be between 1 and 100000")
	ErrInvalidURL        = errors.New("invalid url")
	ErrInvalidMeta       = errors.New("meta must be a JSON object")
	ErrInvalidSubtaskPos = errors.New("position must be >= 0")
)

func NormalizeTitle(input string) string {
	return strings.TrimSpace(input)
}

func NormalizeStatus(input string) string {
	if _, ok := knownStatuses[input]; ok {
		return input
	}
	return StatusInProgress
}

func StatusGlyph(status string) string {
	switch status {
	case StatusCreated:
		return "[ ]"
	case StatusInProgress:
		return "[-]"
	case StatusClosed:
		return "[x]"
	case StatusCompleted:
		return "[✓]"
	case StatusArchived:
		return "[@]"
	default:
		return "[-]"
	}
}

func StatusLabel(status string) string {
	switch status {
	case StatusCreated:
		return "Created"
	case StatusInProgress:
		return "In Progress"
	case StatusClosed:
		return "Closed"
	case StatusCompleted:
		return "Completed"
	case StatusArchived:
		return "Archived"
	default:
		return "In Progress"
	}
}

func IsIncomplete(status string) bool {
	switch status {
	case StatusClosed, StatusCompleted, StatusArchived:
		return false
	case StatusCreated, StatusInProgress:
		return true
	default:
		return true
	}
}

func CycleStatus(status string) string {
	current := NormalizeStatus(status)
	for index, value := range statusCycle {
		if value == current {
			return statusCycle[(index+1)%len(statusCycle)]
		}
	}
	return StatusCompleted
}

func ValidateTask(task *Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	task.Title = NormalizeTitle(task.Title)
	if task.Title == "" || len(task.Title) > 256 {
		return ErrInvalidTitle
	}
	task.Status = NormalizeStatus(task.Status)
	normalizedURL, err := NormalizeURL(task.URL)
	if err != nil {
		return err
	}
	task.URL = normalizedURL
	if task.Priority != nil {
		if *task.Priority < 1 || *task.Priority > 4 {
			return ErrInvalidPriority
		}
	}
	if task.Progress != nil {
		if *task.Progress < 0 || *task.Progress > 100 {
			return ErrInvalidProgress
		}
	}
	if task.EstimatedMinutes != nil {
		if *task.EstimatedMinutes < 1 || *task.EstimatedMinutes > 100000 {
			return ErrInvalidEstimate
		}
	}
	if task.Meta == nil {
		task.Meta = map[string]any{}
	}
	if err := ValidateMeta(task.Meta); err != nil {
		return err
	}
	return nil
}

func ValidateSubtask(subtask *Subtask) error {
	if subtask == nil {
		return errors.New("subtask is nil")
	}
	subtask.Title = NormalizeTitle(subtask.Title)
	if subtask.Title == "" || len(subtask.Title) > 256 {
		return ErrInvalidTitle
	}
	if subtask.Position < 0 {
		return ErrInvalidSubtaskPos
	}
	subtask.Status = NormalizeStatus(subtask.Status)
	if subtask.Meta == nil {
		subtask.Meta = map[string]any{}
	}
	if err := ValidateMeta(subtask.Meta); err != nil {
		return err
	}
	return nil
}

func NormalizeURL(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil
	}
	candidate := trimmed
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if parsed.Scheme == "" {
		return "", fmt.Errorf("%w: missing scheme", ErrInvalidURL)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if _, err := url.ParseRequestURI(candidate); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	return parsed.String(), nil
}

func ValidateMeta(meta map[string]any) error {
	if meta == nil {
		return nil
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMeta, err)
	}
	var result any
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMeta, err)
	}
	if _, ok := result.(map[string]any); !ok {
		return ErrInvalidMeta
	}
	return nil
}

func ParseMetaJSON(input string) (map[string]any, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("parse meta json: %w", err)
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return nil, ErrInvalidMeta
	}
	return object, nil
}

func MustMetaJSON(meta map[string]any) string {
	if meta == nil {
		return "{}"
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func ParseIntPointer(input string) (*int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
