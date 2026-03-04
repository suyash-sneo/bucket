package components

import (
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func RenderToast(palette theme.Theme, message string) string {
	if message == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(palette.BG).Background(palette.Accent).Padding(0, 1).Render(message)
}
