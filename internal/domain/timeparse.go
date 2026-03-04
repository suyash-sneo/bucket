package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DueDateLayout     = "2006-01-02"
	DueDateTimeLayout = "2006-01-02 15:04"
)

func ParseDueInput(input string, location *time.Location) (*time.Time, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	if location == nil {
		location = time.Local
	}

	if len(trimmed) == len(DueDateLayout) {
		parsed, err := time.ParseInLocation(DueDateLayout, trimmed, location)
		if err != nil {
			return nil, fmt.Errorf("parse due date: %w", err)
		}
		return &parsed, nil
	}

	parsed, err := time.ParseInLocation(DueDateTimeLayout, trimmed, location)
	if err != nil {
		return nil, fmt.Errorf("parse due date/time: %w", err)
	}
	return &parsed, nil
}

func FormatDueTime(input *time.Time, location *time.Location) string {
	if input == nil {
		return "(none)"
	}
	if location == nil {
		location = time.Local
	}
	return input.In(location).Format(DueDateTimeLayout)
}

func ParseEstimatedMinutes(input string) (*int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	if numeric, err := strconv.Atoi(trimmed); err == nil {
		if numeric < 1 || numeric > 100000 {
			return nil, ErrInvalidEstimate
		}
		return &numeric, nil
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse estimated duration: %w", err)
	}
	minutes := int(duration.Minutes())
	if minutes < 1 || minutes > 100000 {
		return nil, ErrInvalidEstimate
	}
	return &minutes, nil
}

func HumanizeAgo(now, input time.Time) string {
	if input.IsZero() {
		return "(unknown)"
	}
	diff := now.Sub(input)
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func LocalDayBoundaries(now time.Time) (time.Time, time.Time) {
	local := now.In(time.Local)
	startToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	startTomorrow := startToday.AddDate(0, 0, 1)
	return startToday.UTC(), startTomorrow.UTC()
}
