package components

import (
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func RenderNotesEditor(palette theme.Theme, width, height int, content string) string {
	style := lipgloss.NewStyle().Width(width).Height(height).Foreground(palette.FG)
	return style.Render(content)
}
