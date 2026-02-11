package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// PaletteAction represents a single action in the command palette.
type PaletteAction struct {
	Label        string
	Action       string // "toggle_status", "open_browser", "refresh", "quit"
	Status       string // target status (only for toggle_status)
	SnoozedUntil *int64 // nil except for snooze actions
	Icon         string // display icon/dot
}

// Palette is a command-K style action picker overlay with fuzzy search.
type Palette struct {
	active     bool
	allActions []PaletteAction
	filtered   []PaletteAction
	cursor     int
	convID     int
	input      textinput.Model
}

func NewPalette() Palette {
	return Palette{}
}

func (p *Palette) Open(convID int, currentStatus string) tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Prompt = "> "
	ti.PromptStyle = filterStyle
	ti.Focus()

	p.active = true
	p.convID = convID
	p.cursor = 0
	p.input = ti

	// Build actions based on current status
	p.allActions = nil

	// Status actions
	if currentStatus != "open" {
		p.allActions = append(p.allActions, PaletteAction{Label: "Reopen", Action: "toggle_status", Status: "open", Icon: statusDot("open")})
	}
	if currentStatus != "resolved" {
		p.allActions = append(p.allActions, PaletteAction{Label: "Mark as resolved", Action: "toggle_status", Status: "resolved", Icon: statusDot("resolved")})
	}
	if currentStatus != "pending" {
		p.allActions = append(p.allActions, PaletteAction{Label: "Mark as pending", Action: "toggle_status", Status: "pending", Icon: statusDot("pending")})
	}
	if currentStatus != "snoozed" {
		dot := statusDot("snoozed")
		p.allActions = append(p.allActions,
			PaletteAction{Label: "Snooze until next reply", Action: "toggle_status", Status: "snoozed", SnoozedUntil: nil, Icon: dot},
			PaletteAction{Label: "Snooze until tomorrow", Action: "toggle_status", Status: "snoozed", SnoozedUntil: snoozeUntilTomorrow(), Icon: dot},
			PaletteAction{Label: "Snooze until next week", Action: "toggle_status", Status: "snoozed", SnoozedUntil: snoozeUntilNextWeek(), Icon: dot},
		)
	}

	// App actions
	p.allActions = append(p.allActions,
		PaletteAction{Label: "Open in browser", Action: "open_browser", Icon: "→"},
		PaletteAction{Label: "Refresh data", Action: "refresh", Icon: "↻"},
		PaletteAction{Label: "Quit Chatwoot", Action: "quit", Icon: "✕"},
	)

	p.filtered = p.allActions

	return textinput.Blink
}

func (p *Palette) Close() {
	p.active = false
}

func (p *Palette) IsActive() bool {
	return p.active
}

func (p *Palette) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.applyFilter()
	return cmd
}

func (p *Palette) applyFilter() {
	query := p.input.Value()
	if query == "" {
		p.filtered = p.allActions
	} else {
		labels := make([]string, len(p.allActions))
		for i, a := range p.allActions {
			labels[i] = a.Label
		}
		matches := fuzzy.Find(query, labels)
		p.filtered = make([]PaletteAction, len(matches))
		for i, m := range matches {
			p.filtered[i] = p.allActions[m.Index]
		}
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = 0
	}
}

func (p *Palette) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

func (p *Palette) MoveDown() {
	if p.cursor < len(p.filtered)-1 {
		p.cursor++
	}
}

func (p *Palette) Selected() *PaletteAction {
	if len(p.filtered) == 0 {
		return nil
	}
	return &p.filtered[p.cursor]
}

func (p *Palette) ConvID() int {
	return p.convID
}

func (p *Palette) View(termW int) string {
	boxW := termW * 60 / 100
	if boxW < 30 {
		boxW = 30
	}

	textAreaW := boxW - 4 // minus horizontal padding
	p.input.Width = textAreaW - 2 // minus prompt "> "
	header := lipgloss.NewStyle().
		Width(textAreaW).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(false).BorderLeft(false).BorderRight(false).BorderBottom(true).
		BorderForeground(colorBorder).
		Render(p.input.View())

	var b strings.Builder
	for i, action := range p.filtered {
		label := "  " + action.Icon + " " + action.Label
		if i == p.cursor {
			label = lipgloss.NewStyle().Bold(true).Background(colorSelected).
				Render("> " + action.Icon + " " + action.Label)
		}
		b.WriteString(label)
		if i < len(p.filtered)-1 {
			b.WriteString("\n")
		}
	}

	actionList := b.String()
	if len(p.filtered) == 0 {
		actionList = lipgloss.NewStyle().Foreground(colorMuted).Render("  No matching actions")
	}

	footer := lipgloss.NewStyle().Foreground(colorMuted).Render("↑↓ navigate  Enter select  Esc cancel")

	content := header + "\n\n" + actionList + "\n\n" + footer

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(boxW).
		Render(content)
}

func snoozeUntilTomorrow() *int64 {
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
	ts := tomorrow.Unix()
	return &ts
}

func snoozeUntilNextWeek() *int64 {
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	nextMonday := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 9, 0, 0, 0, now.Location())
	ts := nextMonday.Unix()
	return &ts
}
