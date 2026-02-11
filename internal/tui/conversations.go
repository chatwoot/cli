package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	"github.com/sahilm/fuzzy"
)

var (
	assigneeTabs   = []string{"me", "unassigned", "all"}
	tabLabels      = []string{"Mine", "Unassigned", "All"}
	statusOptions  = []string{"open", "resolved", "pending", "snoozed"}
	statusLabels   = []string{"Open", "Resolved", "Pending", "Snoozed"}
)

type ConversationList struct {
	conversations []sdk.Conversation
	filtered      []sdk.Conversation
	cursor        int
	width, height int
	scrollOffset  int

	tabIndex    int // index into assigneeTabs
	statusIndex int // index into statusOptions
	filtering   bool
	filterInput textinput.Model
}

func NewConversationList() ConversationList {
	ti := textinput.New()
	ti.Placeholder = "Search conversations..."
	ti.CharLimit = 100
	return ConversationList{
		filterInput: ti,
	}
}

func (c *ConversationList) StatusFilter() string {
	return statusOptions[c.statusIndex]
}

func (c *ConversationList) AssigneeType() string {
	return assigneeTabs[c.tabIndex]
}

func (c *ConversationList) SetSize(w, h int) {
	c.width = w
	c.height = h
}

func (c *ConversationList) SetConversations(convs []sdk.Conversation) {
	c.conversations = convs
	c.applyFilter()
}

func (c *ConversationList) Selected() *sdk.Conversation {
	if len(c.filtered) == 0 {
		return nil
	}
	if c.cursor >= len(c.filtered) {
		c.cursor = len(c.filtered) - 1
	}
	return &c.filtered[c.cursor]
}

func (c *ConversationList) applyFilter() {
	query := c.filterInput.Value()
	if query == "" {
		c.filtered = c.conversations
		return
	}

	strs := make([]string, len(c.conversations))
	for i, conv := range c.conversations {
		name := ""
		if conv.Meta.Sender != nil {
			name = conv.Meta.Sender.Name
		}
		snippet := ""
		if len(conv.Messages) > 0 {
			snippet = conv.Messages[0].Content
		}
		strs[i] = fmt.Sprintf("%d %s %s %s", conv.ID, name, conv.Status, snippet)
	}

	matches := fuzzy.Find(query, strs)
	c.filtered = make([]sdk.Conversation, len(matches))
	for i, m := range matches {
		c.filtered[i] = c.conversations[m.Index]
	}

	if c.cursor >= len(c.filtered) {
		c.cursor = 0
	}
}

func (c *ConversationList) Update(msg tea.Msg) tea.Cmd {
	if c.filtering {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				c.filtering = false
				c.filterInput.Blur()
				c.filterInput.SetValue("")
				c.applyFilter()
				return nil
			case "enter":
				c.filtering = false
				c.filterInput.Blur()
				return nil
			}
		}
		var cmd tea.Cmd
		c.filterInput, cmd = c.filterInput.Update(msg)
		c.applyFilter()
		return cmd
	}
	return nil
}

func (c *ConversationList) MoveUp() {
	if c.cursor > 0 {
		c.cursor--
		if c.cursor < c.scrollOffset {
			c.scrollOffset = c.cursor
		}
	}
}

func (c *ConversationList) MoveDown() {
	if c.cursor < len(c.filtered)-1 {
		c.cursor++
		visible := c.visibleRows()
		if c.cursor >= c.scrollOffset+visible {
			c.scrollOffset = c.cursor - visible + 1
		}
	}
}

func (c *ConversationList) CycleTab() {
	c.tabIndex = (c.tabIndex + 1) % len(assigneeTabs)
	c.cursor = 0
	c.scrollOffset = 0
}

func (c *ConversationList) CycleStatus() {
	c.statusIndex = (c.statusIndex + 1) % len(statusOptions)
	c.cursor = 0
	c.scrollOffset = 0
}

func (c *ConversationList) StartFilter() {
	c.filtering = true
	c.filterInput.Focus()
}

func (c *ConversationList) IsFiltering() bool {
	return c.filtering
}

func (c *ConversationList) visibleRows() int {
	// 2 header lines (tabs + filter), each row = 1 line
	available := c.height - 2
	if available < 1 {
		return 1
	}
	return available
}

func (c *ConversationList) View() string {
	var b strings.Builder

	// Assignee tabs: Mine | Unassigned | All
	var tabs []string
	for i, label := range tabLabels {
		if i == c.tabIndex {
			tabs = append(tabs, statusTabActive.Render(label))
		} else {
			tabs = append(tabs, statusTabInactive.Render(label))
		}
	}
	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	// Status indicator: just dot + label, no "Status:" prefix
	statusLabel := " " + statusDot(statusOptions[c.statusIndex]) + " " +
		lipgloss.NewStyle().Bold(true).Render(statusLabels[c.statusIndex])
	b.WriteString(tabLine + statusLabel)
	b.WriteString("\n")

	// Filter line
	if c.filtering {
		b.WriteString(filterStyle.Render("/ ") + c.filterInput.View())
	} else if c.filterInput.Value() != "" {
		b.WriteString(filterStyle.Render("filter: " + c.filterInput.Value()))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("/ to filter"))
	}
	b.WriteString("\n")

	// Conversation rows
	visible := c.visibleRows()
	end := c.scrollOffset + visible
	if end > len(c.filtered) {
		end = len(c.filtered)
	}

	if len(c.filtered) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  No conversations"))
		b.WriteString("\n")
	}

	for i := c.scrollOffset; i < end; i++ {
		conv := c.filtered[i]
		selected := i == c.cursor
		if i > c.scrollOffset {
			b.WriteString("\n")
		}
		b.WriteString(c.renderRow(conv, selected))
	}

	return b.String()
}

func (c *ConversationList) renderRow(conv sdk.Conversation, selected bool) string {
	name := ""
	if conv.Meta.Sender != nil {
		name = conv.Meta.Sender.Name
	}
	if name == "" {
		name = "Unknown"
	}

	ts := formatTime(conv.LastActivityAt)
	idStr := fmt.Sprintf("#%d", conv.ID)
	dot := statusDot(conv.Status)

	// Cursor(2) + dot(1) + space(1) + id + space(1) + name + space(1) + ts
	prefix := "  "
	if selected {
		prefix = "> "
	}

	fixedW := 2 + 1 + 1 + len(idStr) + 1 + 1 + len(ts) // prefix + dot + spaces + id + ts
	nameW := c.width - fixedW
	if nameW < 3 {
		nameW = 3
	}
	truncName := truncate(name, nameW)
	padded := truncName + strings.Repeat(" ", nameW-len([]rune(truncName)))

	line := prefix + dot + " " + idStr + " " + padded + " " + ts

	if selected {
		line = convSelectedStyle.Render(line)
	}
	return line
}

func formatTime(epoch int64) string {
	if epoch == 0 {
		return ""
	}
	t := time.Unix(epoch, 0)
	now := time.Now()
	if t.YearDay() == now.YearDay() && t.Year() == now.Year() {
		return t.Format("3:04 PM")
	}
	return t.Format("Jan 2")
}

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-3]) + "..."
}

