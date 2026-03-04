package components

import (
	"bucket/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func RenderFooterHelp(palette theme.Theme, width int, helpText string) string {
	if width <= 0 {
		return ""
	}
	line := runewidth.Truncate(helpText, width, "…")
	style := lipgloss.NewStyle().Foreground(palette.FooterFG)
	return style.Render(line)
}
