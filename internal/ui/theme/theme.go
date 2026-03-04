package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name        string
	BG          lipgloss.Color
	FG          lipgloss.Color
	Muted       lipgloss.Color
	Accent      lipgloss.Color
	Accent2     lipgloss.Color
	Error       lipgloss.Color
	Success     lipgloss.Color
	Warn        lipgloss.Color
	SelectionBG lipgloss.Color
	SelectionFG lipgloss.Color
	HeaderFG    lipgloss.Color
	FooterFG    lipgloss.Color
}

func Resolve(configTheme string) Theme {
	switch configTheme {
	case "dark":
		return Dark()
	case "light":
		return Light()
	default:
		if lipgloss.HasDarkBackground() {
			return Dark()
		}
		return Light()
	}
}

func Dark() Theme {
	return Theme{
		Name:        "dark",
		BG:          lipgloss.Color("#121212"),
		FG:          lipgloss.Color("#E8E8E8"),
		Muted:       lipgloss.Color("#8B8B8B"),
		Accent:      lipgloss.Color("#7AA2F7"),
		Accent2:     lipgloss.Color("#89DDFF"),
		Error:       lipgloss.Color("#F7768E"),
		Success:     lipgloss.Color("#9ECE6A"),
		Warn:        lipgloss.Color("#E0AF68"),
		SelectionBG: lipgloss.Color("#2F334D"),
		SelectionFG: lipgloss.Color("#FFFFFF"),
		HeaderFG:    lipgloss.Color("#C0CAF5"),
		FooterFG:    lipgloss.Color("#A9B1D6"),
	}
}

func Light() Theme {
	return Theme{
		Name:        "light",
		BG:          lipgloss.Color("#FAFAFA"),
		FG:          lipgloss.Color("#222222"),
		Muted:       lipgloss.Color("#6B6B6B"),
		Accent:      lipgloss.Color("#1E66F5"),
		Accent2:     lipgloss.Color("#179299"),
		Error:       lipgloss.Color("#D20F39"),
		Success:     lipgloss.Color("#40A02B"),
		Warn:        lipgloss.Color("#DF8E1D"),
		SelectionBG: lipgloss.Color("#DCE6FF"),
		SelectionFG: lipgloss.Color("#111111"),
		HeaderFG:    lipgloss.Color("#34495E"),
		FooterFG:    lipgloss.Color("#5A6473"),
	}
}
