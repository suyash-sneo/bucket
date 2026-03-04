package components

import (
	"strings"

	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func RenderModal(palette theme.Theme, width, height int, title, body string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	modalWidth := width - 6
	if modalWidth < 20 {
		modalWidth = width
	}
	content := lipgloss.NewStyle().Bold(true).Foreground(palette.Error).Render(title) + "\n\n" + body + "\n\nEsc to dismiss"
	box := lipgloss.NewStyle().Padding(1, 2).Width(modalWidth).Foreground(palette.FG).Background(palette.BG).Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, strings.TrimRight(box, "\n"))
}
