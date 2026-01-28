package tui

import (
	"fmt"
	"strings"

	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	"github.com/charmbracelet/lipgloss"
)

type Tab int

const (
	TabMine Tab = iota
	TabAll
)

type ConversationsList struct {
	client        *sdk.Client
	conversations []sdk.Conversation
	cursor        int
	loading       bool
	err           error
	width         int
	height        int
	ActiveTab     Tab
}

func NewConversationsList(client *sdk.Client) *ConversationsList {
	return &ConversationsList{
		client:        client,
		conversations: []sdk.Conversation{},
		cursor:        0,
		loading:       false,
		ActiveTab:     TabMine,
	}
}

func (c *ConversationsList) FetchConversations() {
	c.loading = true
	c.err = nil
	c.cursor = 0

	opts := sdk.ListOptions{
		Status: "open",
		SortBy: "last_activity_at_desc",
	}

	if c.ActiveTab == TabMine {
		opts.AssigneeType = "me"
	}

	resp, err := c.client.Conversations().List(opts)
	if err != nil {
		c.err = err
		c.loading = false
		return
	}

	c.conversations = resp.Data.Payload
	c.loading = false
}

func (c *ConversationsList) NextTab() {
	if c.ActiveTab == TabMine {
		c.ActiveTab = TabAll
	} else {
		c.ActiveTab = TabMine
	}
	c.FetchConversations()
}

func (c *ConversationsList) PrevTab() {
	c.NextTab() // Only 2 tabs, so same as next
}

func (c *ConversationsList) MoveUp() {
	if c.cursor > 0 {
		c.cursor--
	}
}

func (c *ConversationsList) MoveDown() {
	if c.cursor < len(c.conversations)-1 {
		c.cursor++
	}
}

func (c *ConversationsList) Selected() *sdk.Conversation {
	if len(c.conversations) == 0 {
		return nil
	}
	return &c.conversations[c.cursor]
}

func (c *ConversationsList) SetSize(width, height int) {
	c.width = width
	c.height = height
}

func (c *ConversationsList) View() string {
	// Tabs
	tabs := c.renderTabs()

	// Content
	var content string
	if c.loading {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Italic(true).
			Render("Loading...")
	} else if c.err != nil {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Render(fmt.Sprintf("Error: %v", c.err))
	} else if len(c.conversations) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Render("No open conversations")
	} else {
		content = c.renderList()
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	help := helpStyle.Render("j/k nav • 1/2 tabs • r refresh")

	return fmt.Sprintf("%s\n%s\n%s", tabs, content, help)
}

func (c *ConversationsList) renderTabs() string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Background(lipgloss.Color("#3C3C3C")).
		Padding(0, 2)

	var mineTab, allTab string
	if c.ActiveTab == TabMine {
		mineTab = activeStyle.Render("Mine")
		allTab = inactiveStyle.Render("All")
	} else {
		mineTab = inactiveStyle.Render("Mine")
		allTab = activeStyle.Render("All")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, mineTab, allTab)
}

func (c *ConversationsList) renderList() string {
	// Each conversation takes 3 lines now (name, meta, message)
	maxItems := (c.height - 6) / 3
	if maxItems < 1 {
		maxItems = 3
	}

	visibleStart := c.cursor - (maxItems / 2)
	if visibleStart < 0 {
		visibleStart = 0
	}
	visibleEnd := visibleStart + maxItems
	if visibleEnd > len(c.conversations) {
		visibleEnd = len(c.conversations)
		visibleStart = visibleEnd - maxItems
		if visibleStart < 0 {
			visibleStart = 0
		}
	}

	var lines []string
	for i := visibleStart; i < visibleEnd; i++ {
		conv := c.conversations[i]
		lines = append(lines, c.renderConversation(conv, i == c.cursor))
	}

	return strings.Join(lines, "\n")
}

func (c *ConversationsList) renderConversation(conv sdk.Conversation, selected bool) string {
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF69B4")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA"))

	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262"))

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	senderName := "Unknown"
	if conv.Meta.Sender != nil && conv.Meta.Sender.Name != "" {
		senderName = conv.Meta.Sender.Name
	}

	inboxName := conv.Meta.Channel

	// Get last message preview
	lastMessage := ""
	if len(conv.Messages) > 0 {
		msg := conv.Messages[len(conv.Messages)-1]
		lastMessage = truncate(msg.Content, c.width-6)
	}

	line := fmt.Sprintf("#%d %s", conv.ID, truncate(senderName, 15))
	metaLine := fmt.Sprintf("%s (%d)", inboxName, conv.MessagesCount)

	if selected {
		return fmt.Sprintf("%s\n  %s\n  %s",
			selectedStyle.Render("> "+line),
			metaStyle.Render(metaLine),
			messageStyle.Render(lastMessage))
	}
	return fmt.Sprintf("%s\n  %s\n  %s",
		normalStyle.Render("  "+line),
		metaStyle.Render(metaLine),
		messageStyle.Render(lastMessage))
}

func truncate(s string, max int) string {
	// Replace newlines with spaces for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
