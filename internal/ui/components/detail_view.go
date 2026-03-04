package components

import (
	"fmt"
	"strings"
	"time"

	"bucket/internal/domain"
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func RenderDetailView(palette theme.Theme, width, height int, task domain.Task, subtasks []domain.Subtask, notesPreview string, now time.Time) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(palette.Accent).Render(task.Title),
		fmt.Sprintf("%s  %s", domain.StatusGlyph(task.Status), domain.StatusLabel(task.Status)),
		fmt.Sprintf("URL: %s", lipgloss.NewStyle().Foreground(palette.Accent2).Render(CompactURL(task.URL, maxInt(8, width-8)))),
		fmt.Sprintf("Due: %s", domain.FormatDueTime(task.DueAt, time.Local)),
		fmt.Sprintf("Priority: %s", intPtrOrNone(task.Priority)),
		fmt.Sprintf("Estimate: %s", minutesOrNone(task.EstimatedMinutes)),
		fmt.Sprintf("Progress: %s", progressOrNone(task.Progress)),
		fmt.Sprintf("Created: %s", domain.HumanizeAgo(now, task.CreatedAt)),
		fmt.Sprintf("Updated: %s", domain.HumanizeAgo(now, task.UpdatedAt)),
		fmt.Sprintf("Subtasks: %d", len(subtasks)),
	}
	for _, subtask := range subtasks {
		lines = append(lines, fmt.Sprintf("  %s %s", domain.StatusGlyph(subtask.Status), subtask.Title))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(palette.Muted).Render("Notes:"))
	if strings.TrimSpace(notesPreview) == "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(palette.Muted).Render("(none)"))
	} else {
		lines = append(lines, strings.Split(notesPreview, "\n")...)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func intPtrOrNone(input *int) string {
	if input == nil {
		return "(none)"
	}
	return fmt.Sprintf("%d", *input)
}

func minutesOrNone(input *int) string {
	if input == nil {
		return "(none)"
	}
	return fmt.Sprintf("%d min", *input)
}

func progressOrNone(input *int) string {
	if input == nil {
		return "(none)"
	}
	return fmt.Sprintf("%d%%", *input)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
