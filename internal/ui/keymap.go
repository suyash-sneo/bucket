package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit          key.Binding
	QuitAlt       key.Binding
	QuitSIG       key.Binding
	Inbox         key.Binding
	Upcoming      key.Binding
	All           key.Binding
	Closed        key.Binding
	Archived      key.Binding
	Up            key.Binding
	Down          key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	Top           key.Binding
	Bottom        key.Binding
	EnterEdit     key.Binding
	ExitEdit      key.Binding
	QuickAdd      key.Binding
	Cycle         key.Binding
	OpenURL       key.Binding
	Filter        key.Binding
	Apply         key.Binding
	Cancel        key.Binding
	TabNext       key.Binding
	TabPrev       key.Binding
	FocusTitle    key.Binding
	FocusStatus   key.Binding
	FocusURL      key.Binding
	FocusDue      key.Binding
	FocusPriority key.Binding
	FocusEstimate key.Binding
	FocusProgress key.Binding
	FocusSubtasks key.Binding
	FocusNotes    key.Binding
	CycleEdit     key.Binding
	OpenURLEdit   key.Binding
	ClearURL      key.Binding
	SubtaskDelete key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:          key.NewBinding(key.WithKeys("q", "ctrl+q"), key.WithHelp("q", "quit")),
		QuitAlt:       key.NewBinding(key.WithKeys("ctrl+q"), key.WithHelp("ctrl+q", "quit")),
		QuitSIG:       key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Inbox:         key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "Inbox")),
		Upcoming:      key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "Upcoming")),
		All:           key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "All")),
		Closed:        key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "Closed")),
		Archived:      key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "Archived")),
		Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:        key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("PgUp", "page up")),
		PageDown:      key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("PgDn", "page down")),
		Top:           key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:        key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		EnterEdit:     key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "edit")),
		ExitEdit:      key.NewBinding(key.WithKeys("esc", "left", "ctrl+h"), key.WithHelp("esc", "back")),
		QuickAdd:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Cycle:         key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "cycle")),
		OpenURL:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open url")),
		Filter:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Apply:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
		Cancel:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		TabNext:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		TabPrev:       key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		FocusTitle:    key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "title")),
		FocusStatus:   key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "status")),
		FocusURL:      key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "url")),
		FocusDue:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "due")),
		FocusPriority: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "priority")),
		FocusEstimate: key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "estimate")),
		FocusProgress: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "progress")),
		FocusSubtasks: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "subtasks")),
		FocusNotes:    key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "notes")),
		CycleEdit:     key.NewBinding(key.WithKeys("ctrl+space", "ctrl+@"), key.WithHelp("ctrl+space", "cycle")),
		OpenURLEdit:   key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "open url")),
		ClearURL:      key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "clear url")),
		SubtaskDelete: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
	}
}
