package components

import (
	"fmt"
	"strings"
	"unicode"

	"bucket/internal/domain"
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func RenderHeader(palette theme.Theme, width int, listType string, count int, filterState string, selected *domain.TaskListItem, dirty bool) string {
	if width <= 0 {
		return ""
	}
	left := fmt.Sprintf("buckets  •  %s  •  %d tasks  •  %s", strings.Title(listType), count, sanitizeHeaderText(filterState))
	right := ""
	if selected != nil {
		right = fmt.Sprintf("%s %s", domain.StatusGlyph(selected.Status), sanitizeHeaderText(selected.Title))
		right = runewidth.Truncate(right, max(0, width/2), "…")
	}
	if dirty {
		if right == "" {
			right = "Unsaved"
		} else {
			right = "Unsaved • " + right
		}
	}
	leftWidth := width - runewidth.StringWidth(right) - 1
	if leftWidth < 1 {
		leftWidth = width
		right = ""
	}
	left = runewidth.Truncate(left, leftWidth, "…")
	line := left
	if right != "" {
		padding := width - runewidth.StringWidth(left) - runewidth.StringWidth(right)
		if padding < 1 {
			padding = 1
		}
		line += strings.Repeat(" ", padding) + right
	}
	style := lipgloss.NewStyle().Foreground(palette.HeaderFG).Bold(true)
	return style.Render(runewidth.Truncate(line, width, "…"))
}

func sanitizeHeaderText(input string) string {
	clean := strings.Map(func(character rune) rune {
		if character == '\n' || character == '\r' || character == '\t' || unicode.IsControl(character) {
			return ' '
		}
		return character
	}, input)
	return strings.TrimSpace(clean)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
