package components

import (
	"fmt"
	"strings"

	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func RenderEditorForm(palette theme.Theme, width, height int, focused string, values map[string]string, subtaskCount int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	rows := []struct {
		Key   string
		Label string
		Value string
	}{
		{Key: "title", Label: "Title", Value: values["title"]},
		{Key: "status", Label: "Status", Value: values["status"]},
		{Key: "url", Label: "URL", Value: values["url"]},
		{Key: "due", Label: "Due", Value: values["due"]},
		{Key: "priority", Label: "Priority", Value: values["priority"]},
		{Key: "estimate", Label: "Estimated", Value: values["estimate"]},
		{Key: "progress", Label: "Progress", Value: values["progress"]},
		{Key: "subtasks", Label: "Subtasks", Value: fmt.Sprintf("%d (Press ctrl+b to edit)", subtaskCount)},
		{Key: "notes", Label: "Notes", Value: "Press ctrl+n to edit (autosaves every second)"},
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		line := fmt.Sprintf("%-10s %s", row.Label+":", row.Value)
		if row.Key == focused {
			line = lipgloss.NewStyle().Foreground(palette.SelectionFG).Background(palette.SelectionBG).Render(line)
		}
		lines = append(lines, line)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}
