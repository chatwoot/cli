package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

// mentionItem is a unified type for the mention picker (agents + teams).
type mentionItem struct {
	ID         int
	Name       string
	Kind       string // "user" or "team"
	Status     string // availability status (agents only)
	IsTeam     bool
}

// ReplyEditor is a floating modal for composing replies and private notes.
type ReplyEditor struct {
	textarea       textarea.Model
	active         bool
	sending        bool
	private        bool
	conversationID int
	contactName    string

	// Mention picker state
	allMentions    []mentionItem
	mentionActive  bool
	mentionStart   int // index of '@' in textarea value
	mentionIdx     int // selected item in picker
	mentionMatches []mentionItem
}

func NewReplyEditor() ReplyEditor {
	return ReplyEditor{}
}

func (r *ReplyEditor) Open(convID int, contactName string, private bool, width, height int, agents []sdk.AgentFull, teams []sdk.TeamFull) tea.Cmd {
	placeholder := "Type your reply..."
	if private {
		placeholder = "Type your note..."
	}
	ta := textarea.New()
	ta.Placeholder = placeholder
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
	r.private = private
	r.conversationID = convID
	r.contactName = contactName
	r.mentionActive = false

	// Build unified mention list: agents first, then teams
	r.allMentions = nil
	for _, a := range agents {
		r.allMentions = append(r.allMentions, mentionItem{
			ID: a.ID, Name: a.Name, Kind: "user", Status: a.AvailabilityStatus,
		})
	}
	for _, t := range teams {
		r.allMentions = append(r.allMentions, mentionItem{
			ID: t.ID, Name: t.Name, Kind: "team", IsTeam: true,
		})
	}

	return textarea.Blink
}

func (r *ReplyEditor) Close() {
	r.active = false
	r.sending = false
	r.mentionActive = false
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

func (r *ReplyEditor) IsPrivate() bool {
	return r.private
}

func (r *ReplyEditor) ConversationID() int {
	return r.conversationID
}

func (r *ReplyEditor) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	r.textarea, cmd = r.textarea.Update(msg)
	return cmd
}

func (r *ReplyEditor) HasMentions() bool {
	return len(r.allMentions) > 0
}

// --- Mention picker ---

func (r *ReplyEditor) MentionActive() bool {
	return r.mentionActive
}

func (r *ReplyEditor) StartMention() {
	r.mentionActive = true
	r.mentionStart = len(r.textarea.Value()) - 1 // position of the '@' just inserted
	r.mentionIdx = 0
	r.filterMentions()
}

func (r *ReplyEditor) CloseMention() {
	r.mentionActive = false
}

func (r *ReplyEditor) MentionUp() {
	if r.mentionIdx > 0 {
		r.mentionIdx--
	}
}

func (r *ReplyEditor) MentionDown() {
	if r.mentionIdx < len(r.mentionMatches)-1 {
		r.mentionIdx++
	}
}

// CompleteMention replaces @query with the full mention syntax.
func (r *ReplyEditor) CompleteMention() {
	if !r.mentionActive || len(r.mentionMatches) == 0 {
		r.mentionActive = false
		return
	}
	if r.mentionIdx >= len(r.mentionMatches) {
		r.mentionIdx = 0
	}
	item := r.mentionMatches[r.mentionIdx]
	val := r.textarea.Value()

	// mention://user/ID/Name or mention://team/ID/Name
	mention := fmt.Sprintf("[@%s](mention://%s/%d/%s) ", item.Name, item.Kind, item.ID, url.PathEscape(item.Name))
	newVal := val[:r.mentionStart] + mention
	r.textarea.SetValue(newVal)
	r.mentionActive = false
}

// ValidateMention checks if the mention context is still valid after a key press.
func (r *ReplyEditor) ValidateMention() {
	if !r.mentionActive {
		return
	}
	val := r.textarea.Value()
	// The @ must still be in the value at mentionStart
	if r.mentionStart >= len(val) || val[r.mentionStart] != '@' {
		r.mentionActive = false
		return
	}
	// Query is everything after @ to end of value — must not contain whitespace
	query := val[r.mentionStart+1:]
	if strings.ContainsAny(query, " \n\t") {
		r.mentionActive = false
		return
	}
	r.filterMentions()
}

func (r *ReplyEditor) mentionQuery() string {
	if !r.mentionActive {
		return ""
	}
	val := r.textarea.Value()
	if r.mentionStart+1 > len(val) {
		return ""
	}
	return val[r.mentionStart+1:]
}

func (r *ReplyEditor) filterMentions() {
	query := strings.ToLower(r.mentionQuery())
	if query == "" {
		r.mentionMatches = r.allMentions
	} else {
		r.mentionMatches = nil
		for _, m := range r.allMentions {
			if strings.Contains(strings.ToLower(m.Name), query) {
				r.mentionMatches = append(r.mentionMatches, m)
			}
		}
	}
	if r.mentionIdx >= len(r.mentionMatches) {
		r.mentionIdx = max(0, len(r.mentionMatches)-1)
	}
}

// --- View ---

func (r *ReplyEditor) View(termW int) string {
	boxContentW := termW * 60 / 100
	if boxContentW < 30 {
		boxContentW = 30
	}

	accentColor := colorAccent
	headerText := fmt.Sprintf("Replying to %s - #%d", r.contactName, r.conversationID)
	if r.private {
		accentColor = colorPrivate
		headerText = fmt.Sprintf("Private note - #%d", r.conversationID)
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(headerText)

	content := header + "\n\n" + r.textarea.View()

	// Mention picker
	if r.mentionActive && len(r.mentionMatches) > 0 {
		content += "\n" + r.renderMentionPicker()
	}

	var footer string
	if r.sending {
		footer = lipgloss.NewStyle().Foreground(colorMuted).Render("Sending...")
	} else {
		hint := "Ctrl+S send  ·  Esc discard"
		if r.mentionActive {
			hint = "↑↓ select  ·  Tab/Enter pick  ·  Esc cancel"
		}
		footer = lipgloss.NewStyle().Foreground(colorMuted).Render(hint)
	}

	content += "\n\n" + footer

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(boxContentW).
		Render(content)
}

func (r *ReplyEditor) renderMentionPicker() string {
	maxShow := 5
	var lines []string
	for i, item := range r.mentionMatches {
		if i >= maxShow {
			remaining := len(r.mentionMatches) - maxShow
			lines = append(lines, lipgloss.NewStyle().Foreground(colorMuted).
				Render(fmt.Sprintf("  … and %d more", remaining)))
			break
		}

		dot := mentionDot(item)
		label := fmt.Sprintf("  %s %s", dot, item.Name)
		if i == r.mentionIdx {
			label = lipgloss.NewStyle().Bold(true).Background(colorSelected).
				Render(fmt.Sprintf("▸ %s %s", dot, item.Name))
		}
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

func mentionDot(item mentionItem) string {
	if item.IsTeam {
		return lipgloss.NewStyle().Foreground(colorAccent).Render("◆")
	}
	switch item.Status {
	case "online":
		return lipgloss.NewStyle().Foreground(colorOpen).Render("●")
	case "busy":
		return lipgloss.NewStyle().Foreground(colorPending).Render("●")
	default:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("●")
	}
}
