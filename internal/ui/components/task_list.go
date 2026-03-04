package components

import (
	"fmt"
	"strings"

	"bucket/internal/domain"
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func RenderTaskList(palette theme.Theme, width, height int, tasks []domain.TaskListItem, visible []int, selectedIndex int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if len(visible) == 0 {
		visible = make([]int, len(tasks))
		for index := range tasks {
			visible[index] = index
		}
	}
	if len(visible) == 0 {
		return lipgloss.NewStyle().Foreground(palette.Muted).Render("No tasks")
	}

	start := 0
	if selectedIndex >= height {
		start = selectedIndex - height + 1
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(visible) {
		end = len(visible)
	}

	rows := make([]string, 0, height)
	for row := start; row < end; row++ {
		task := tasks[visible[row]]
		line := fmt.Sprintf("%s  %s", domain.StatusGlyph(task.Status), task.Title)
		line = runewidth.Truncate(line, width, "…")
		line = padRunewidth(line, width)
		if row == selectedIndex {
			line = lipgloss.NewStyle().Background(palette.SelectionBG).Foreground(palette.SelectionFG).Render(line)
		} else if task.Status == domain.StatusArchived {
			line = lipgloss.NewStyle().Foreground(palette.Muted).Render(line)
		}
		rows = append(rows, line)
	}
	for len(rows) < height {
		rows = append(rows, strings.Repeat(" ", width))
	}
	return strings.Join(rows, "\n")
}

func padRunewidth(input string, width int) string {
	current := runewidth.StringWidth(input)
	if current >= width {
		return input
	}
	return input + strings.Repeat(" ", width-current)
}
