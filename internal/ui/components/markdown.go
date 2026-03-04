package components

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/charmbracelet/glamour"
)

type MarkdownRenderer struct {
	mu    sync.Mutex
	cache map[string]string
}

func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{cache: map[string]string{}}
}

func (renderer *MarkdownRenderer) Render(taskID int64, notes, themeName string) (string, error) {
	if renderer == nil {
		return notes, nil
	}
	hash := sha1.Sum([]byte(notes))
	key := fmt.Sprintf("%d:%s:%s", taskID, hex.EncodeToString(hash[:]), themeName)

	renderer.mu.Lock()
	if cached, ok := renderer.cache[key]; ok {
		renderer.mu.Unlock()
		return cached, nil
	}
	renderer.mu.Unlock()

	style := "light"
	if themeName == "dark" {
		style = "dracula"
	}
	engine, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(0),
		glamour.WithStandardStyle(style),
	)
	if err != nil {
		return "", fmt.Errorf("build markdown renderer: %w", err)
	}
	output, err := engine.Render(notes)
	if err != nil {
		return "", fmt.Errorf("render notes markdown: %w", err)
	}

	renderer.mu.Lock()
	renderer.cache[key] = output
	renderer.mu.Unlock()
	return output, nil
}
