package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Back    key.Binding
	Filter  key.Binding
	Tab     key.Binding
	Status  key.Binding
	Refresh key.Binding
	Open    key.Binding
	Reply   key.Binding
	Help    key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter", "right"),
		key.WithHelp("enter/→", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("left", "esc"),
		key.WithHelp("←/esc", "back"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "assignee"),
	),
	Status: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "status"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
	Reply: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "reply"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

func helpText(hasSelection bool) string {
	text := "↑↓ navigate  / filter  Tab mine/unassigned/all  s status  "
	if hasSelection {
		text += "R reply  o open  "
	}
	text += "r refresh  q quit"
	return helpBarStyle.Render(text)
}
