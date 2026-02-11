package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReplyEditor is a floating modal for composing replies.
type ReplyEditor struct {
	textarea       textarea.Model
	active         bool
	sending        bool
	conversationID int
	contactName    string
}

func NewReplyEditor() ReplyEditor {
	return ReplyEditor{}
}

func (r *ReplyEditor) Open(convID int, contactName string, width, height int) tea.Cmd {
	ta := textarea.New()
	ta.Placeholder = "Type your reply..."
	ta.CharLimit = 0
	ta.ShowLineNumbers = false

	// Size the textarea: 60% of terminal width, ~40% height
	boxContentW := width * 60 / 100
	if boxContentW < 30 {
		boxContentW = 30
	}
	// Account for box border(2) + padding(4) on each side
	taW := boxContentW - 6
	if taW < 20 {
		taW = 20
	}
	taH := height * 40 / 100
	if taH < 3 {
		taH = 3
	}

	ta.SetWidth(taW)
	ta.SetHeight(taH)
	ta.Focus()

	r.textarea = ta
	r.active = true
	r.sending = false
	r.conversationID = convID
	r.contactName = contactName
	return textarea.Blink
}

func (r *ReplyEditor) Close() {
	r.active = false
	r.sending = false
}

func (r *ReplyEditor) IsActive() bool {
	return r.active
}

func (r *ReplyEditor) IsSending() bool {
	return r.sending
}

func (r *ReplyEditor) SetSending() {
	r.sending = true
}

func (r *ReplyEditor) Value() string {
	return r.textarea.Value()
}

func (r *ReplyEditor) ConversationID() int {
	return r.conversationID
}

func (r *ReplyEditor) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	r.textarea, cmd = r.textarea.Update(msg)
	return cmd
}

func (r *ReplyEditor) View(termW, termH int) string {
	boxContentW := termW * 60 / 100
	if boxContentW < 30 {
		boxContentW = 30
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).
		Render(fmt.Sprintf("Replying to %s - #%d", r.contactName, r.conversationID))

	var footer string
	if r.sending {
		footer = lipgloss.NewStyle().Foreground(colorMuted).Render("Sending...")
	} else {
		footer = lipgloss.NewStyle().Foreground(colorMuted).Render("Ctrl+S send  ·  Esc discard")
	}

	content := header + "\n\n" + r.textarea.View() + "\n\n" + footer

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(boxContentW)

	box := boxStyle.Render(content)

	return lipgloss.Place(termW, termH, lipgloss.Center, lipgloss.Center, box)
}
