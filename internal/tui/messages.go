package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	"github.com/muesli/termenv"
)

// chatStyleConfig returns a glamour style tuned for chat bubbles: no document
// margin/indent and no block prefix/suffix newlines. Detected once at startup
// before bubbletea takes over the terminal (WithAutoStyle deadlocks).
var chatStyleConfig = func() glamour.TermRendererOption {
	var cfg ansi.StyleConfig
	if termenv.HasDarkBackground() {
		cfg = styles.DarkStyleConfig
	} else {
		cfg = styles.LightStyleConfig
	}
	// Strip document-level whitespace — content lives inside a lipgloss box
	zero := uint(0)
	cfg.Document.Margin = &zero
	cfg.Document.BlockPrefix = ""
	cfg.Document.BlockSuffix = ""
	return glamour.WithStyles(cfg)
}()

// MessagePane renders the messages for the selected conversation.
// Messages are only loaded when the user presses Enter.
type MessagePane struct {
	messages         []sdk.Message
	conversationID   int // which conversation messages belong to
	loaded           bool
	loadingMore      bool
	hasMoreMessages  bool
	oldestMessageID  int
	width, height    int
	scrollOffset     int
	mdRenderer       *glamour.TermRenderer
}

func NewMessagePane() MessagePane {
	return MessagePane{}
}

func (p *MessagePane) SetSize(w, h int) {
	p.width = w
	p.height = h

	// Recreate markdown renderer when width changes
	boxContentW := w * 70 / 100
	if boxContentW < 15 {
		boxContentW = 15
	}
	textW := boxContentW - 2
	if textW < 5 {
		textW = 5
	}
	r, err := glamour.NewTermRenderer(
		chatStyleConfig,
		glamour.WithWordWrap(textW),
	)
	if err == nil {
		p.mdRenderer = r
	}
}

func (p *MessagePane) SetMessages(convID int, msgs []sdk.Message) {
	p.conversationID = convID
	p.messages = msgs
	p.loaded = true
	p.loadingMore = false

	// Track oldest message ID for pagination
	if len(msgs) > 0 {
		p.oldestMessageID = msgs[0].ID
		// Assume more messages if we got a full page (20+)
		p.hasMoreMessages = len(msgs) >= 20
	} else {
		p.hasMoreMessages = false
	}

	p.scrollToBottom()
}

func (p *MessagePane) Clear() {
	p.messages = nil
	p.conversationID = 0
	p.loaded = false
	p.scrollOffset = 0
}

func (p *MessagePane) IsLoaded() bool {
	return p.loaded
}

func (p *MessagePane) ConversationID() int {
	return p.conversationID
}

func (p *MessagePane) PrependMessages(msgs []sdk.Message) {
	if len(msgs) == 0 {
		p.hasMoreMessages = false
		p.loadingMore = false
		return
	}

	// Calculate current scroll position in lines before prepending
	linesBefore := p.countLines()

	// Prepend older messages
	p.messages = append(msgs, p.messages...)
	p.oldestMessageID = msgs[0].ID
	p.hasMoreMessages = len(msgs) >= 20
	p.loadingMore = false

	// Adjust scroll offset to maintain visual position
	linesAfter := p.countLines()
	p.scrollOffset += linesAfter - linesBefore
}

func (p *MessagePane) ShouldLoadMore() bool {
	return p.loaded && !p.loadingMore && p.hasMoreMessages && p.scrollOffset < 10
}

func (p *MessagePane) OldestMessageID() int {
	return p.oldestMessageID
}

func (p *MessagePane) SetLoadingMore() {
	p.loadingMore = true
}

func (p *MessagePane) scrollToBottom() {
	total := p.countLines()
	if total > p.height {
		p.scrollOffset = total - p.height
	} else {
		p.scrollOffset = 0
	}
}

func (p *MessagePane) ScrollUp() {
	if p.scrollOffset > 0 {
		p.scrollOffset--
	}
}

func (p *MessagePane) ScrollDown() {
	total := p.countLines()
	if total > p.height && p.scrollOffset < total-p.height {
		p.scrollOffset++
	}
}

func (p *MessagePane) countLines() int {
	n := 0
	for _, msg := range p.messages {
		n += len(p.renderMessage(msg))
	}
	return n
}

// View renders the message pane content.
// CRITICAL: output is strictly bounded to p.height lines to prevent overflow.
func (p *MessagePane) View() string {
	if !p.loaded {
		logo := lipgloss.NewStyle().Foreground(lipgloss.Color("#1f93ff")).Render(chatwootLogo)
		hint := lipgloss.NewStyle().Foreground(colorMuted).Render("Press Enter to load messages")
		return lipgloss.NewStyle().
			Width(p.width).Height(p.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(logo + "\n\n" + hint)
	}

	if len(p.messages) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(p.width).Height(p.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No messages")
	}

	// Render each message into lines, collect into flat slice.
	var lines []string
	for _, msg := range p.messages {
		rendered := p.renderMessage(msg)
		lines = append(lines, rendered...)
	}
	// Apply scroll and clamp to height
	if p.scrollOffset > len(lines)-p.height {
		p.scrollOffset = len(lines) - p.height
	}
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}

	end := p.scrollOffset + p.height
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[p.scrollOffset:end]

	return strings.Join(visible, "\n")
}

// renderMessage returns lines for a bordered message box + 1 blank separator.
// Box width = 70% of pane width. Incoming = left, outgoing = right, activity = centered.
func (p *MessagePane) renderMessage(msg sdk.Message) []string {
	paneW := p.width
	boxContentW := paneW * 70 / 100
	if boxContentW < 15 {
		boxContentW = 15
	}
	// Box border adds 2 to visual width, padding(0,1) adds 2 more = +4 visual
	// So boxContentW is what we pass to Width(), visual = boxContentW + 2 (border only, no padding on box)
	// We use Padding(0,1) so inner text width = boxContentW - 2
	textW := boxContentW - 2
	if textW < 5 {
		textW = 5
	}

	// Activity messages: no box, centered muted text
	if msg.MessageType == 2 {
		content := msg.Content
		if content == "" {
			content = "(activity)"
		}
		line := lipgloss.NewStyle().
			Foreground(colorMuted).Italic(true).
			Width(paneW).Align(lipgloss.Center).
			Render(truncate(content, paneW-2))
		return []string{line, ""}
	}

	sender := "Unknown"
	if msg.Sender != nil {
		sender = msg.Sender.Name
	}

	ts := formatTime(msg.CreatedAt)
	statusIcon := msgStatus(msg.Status, msg.MessageType)

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = "(no content)"
	}

	// Render markdown
	if p.mdRenderer != nil {
		rendered, err := p.mdRenderer.Render(content)
		if err == nil {
			content = strings.TrimRight(rendered, "\n")
		}
	}

	// Metadata line below box: [sender ·] #ID · time [· status]
	// Skip sender for incoming messages (type 0) — visible in info pane
	var metaParts []string
	if msg.MessageType != 0 {
		metaParts = append(metaParts, sender)
	}
	metaParts = append(metaParts, fmt.Sprintf("#%d", msg.ID), ts)
	if statusIcon != "" {
		metaParts = append(metaParts, statusIcon)
	}
	meta := strings.Join(metaParts, " · ")

	metaStyle := lipgloss.NewStyle().Foreground(colorMuted)

	// Build the box (content only)
	var borderColor lipgloss.AdaptiveColor
	if msg.Private {
		borderColor = colorPrivate
	} else if msg.MessageType == 1 {
		borderColor = colorOutgoing
	} else {
		borderColor = colorBorder
	}

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(boxContentW)

	box := boxStyle.Render(content)

	// Position: left or right
	if msg.MessageType == 1 {
		// Outgoing: right-aligned
		boxLine := lipgloss.NewStyle().Width(paneW).Align(lipgloss.Right).Render(box)
		boxLines := strings.Split(boxLine, "\n")
		metaLine := lipgloss.NewStyle().Width(paneW).Align(lipgloss.Right).
			Render(metaStyle.Render(meta))
		result := append(boxLines, metaLine, "") // box + meta + separator
		return result
	}

	// Incoming: left-aligned
	boxLines := strings.Split(box, "\n")
	metaLine := metaStyle.Render(meta)
	result := append(boxLines, metaLine, "") // box + meta + separator
	return result
}

func msgStatus(status string, messageType int) string {
	// Only show status for outgoing messages
	if messageType != 1 {
		return ""
	}
	switch status {
	case "sent":
		return lipgloss.NewStyle().Foreground(colorMuted).Render("✔︎")
	case "delivered":
		return lipgloss.NewStyle().Foreground(colorMuted).Render("✔︎✔︎")
	case "read":
		return lipgloss.NewStyle().Foreground(colorAccent).Render("✔︎✔︎")
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("●")
	default:
		return ""
	}
}
