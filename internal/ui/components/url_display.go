package components

import (
	"net/url"
	"strings"

	"github.com/mattn/go-runewidth"
)

func CompactURL(input string, maxWidth int) string {
	cleaned := strings.TrimSpace(input)
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")
	if cleaned == "" {
		return "(none)"
	}

	preview := cleaned
	if parsed, err := url.Parse(cleaned); err == nil && parsed.Host != "" {
		preview = parsed.Host
		if parsed.Path != "" && parsed.Path != "/" {
			preview += parsed.Path
		}
		if parsed.RawQuery != "" {
			preview += "?" + parsed.RawQuery
		}
		if parsed.Fragment != "" {
			preview += "#" + parsed.Fragment
		}
	} else {
		preview = strings.TrimPrefix(preview, "https://")
		preview = strings.TrimPrefix(preview, "http://")
	}

	if maxWidth > 0 {
		preview = runewidth.Truncate(preview, maxWidth, "...")
	}
	return preview
}
