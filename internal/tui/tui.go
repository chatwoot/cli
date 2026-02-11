package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

type Model struct {
	client    *sdk.Client
	accountID int
	version   string
	agentName string

	width  int
	height int

	convList       ConversationList
	msgPane        MessagePane
	activePane     int // 0=conversations, 1=messages
	contact        *sdk.ContactFull
	contactConvID  int // which conversation the contact was fetched for
	loading        bool
	err            error
	spinner        spinner.Model
}

func newModel(client *sdk.Client, accountID int, version string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return Model{
		client:    client,
		accountID: accountID,
		version:   version,
		convList:  NewConversationList(),
		msgPane:   NewMessagePane(),
		spinner:   sp,
		loading:   true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchCmd(),
		fetchProfile(m.client),
		m.spinner.Tick,
		autoRefreshTick(),
	)
}

func (m Model) fetchCmd() tea.Cmd {
	return fetchConversations(m.client, m.convList.StatusFilter(), m.convList.AssigneeType(), 1)
}

// fetchContactIfNeeded returns a command to fetch the contact for the selected
// conversation, or nil if the contact is already loaded for that conversation.
func (m Model) fetchContactIfNeeded() tea.Cmd {
	sel := m.convList.Selected()
	if sel == nil || sel.Meta.Sender == nil {
		return nil
	}
	if m.contactConvID == sel.ID {
		return nil
	}
	return fetchContact(m.client, sel.Meta.Sender.ID)
}

// Layout math:
// Header: 1 line content + 2 border = 3 visual lines
// Footer: 1 line content + 2 border = 3 visual lines
// Body:   remaining height, content = bodyH, visual = bodyH + 2 border
// Total:  3 + (bodyH + 2) + 3 = bodyH + 8
// So:     bodyH = height - 8
//
// Body columns (content widths, border adds 2 each):
// 3 columns × 2 border = 6 border cols
// convW + msgW + infoW + 6 = width
// convW = convPaneWidth, infoW = infoPaneWidth
// msgW = width - convW - infoW - 6

func (m Model) bodyHeight() int {
	h := m.height - 8
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) columnWidths() (convW, msgW, infoW int) {
	convW = convPaneWidth
	infoW = infoPaneWidth
	msgW = m.width - convW - infoW - 6

	// If too narrow for 3 columns, drop info pane
	if msgW < 20 {
		infoW = 0
		msgW = m.width - convW - 4 // 2 panes × 2 border
	}

	// If still too narrow, shrink conv pane
	if msgW < 10 {
		convW = m.width/2 - 2
		msgW = m.width - convW - 4
	}

	return convW, msgW, infoW
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		convW, msgW, _ := m.columnWidths()
		m.convList.SetSize(convW, m.bodyHeight())
		m.msgPane.SetSize(msgW, m.bodyHeight())
		return m, nil

	case profileMsg:
		if msg.err == nil {
			m.agentName = msg.name
		}
		return m, nil

	case conversationsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.convList.SetConversations(msg.conversations)
		return m, m.fetchContactIfNeeded()

	case messagesMsg:
		if msg.err == nil {
			m.msgPane.SetMessages(msg.conversationID, msg.messages)
		}
		return m, nil

	case contactMsg:
		if msg.err == nil && msg.contact != nil {
			m.contact = msg.contact
			if sel := m.convList.Selected(); sel != nil {
				m.contactConvID = sel.ID
			}
		}
		return m, nil

	case tickMsg:
		m.loading = true
		return m, tea.Batch(m.fetchCmd(), autoRefreshTick(), m.spinner.Tick)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.convList.IsFiltering() {
		cmd := m.convList.Update(msg)
		return m, cmd
	}

	// Global keys (work in any pane)
	switch {
	case matchKey(msg, keys.Quit):
		return m, tea.Quit
	case matchKey(msg, keys.Refresh):
		m.loading = true
		return m, tea.Batch(m.fetchCmd(), m.spinner.Tick)
	case matchKey(msg, keys.Open):
		if sel := m.convList.Selected(); sel != nil {
			url := fmt.Sprintf("%s/app/accounts/%d/conversations/%d",
				m.client.BaseURL, m.accountID, sel.ID)
			openBrowser(url)
		}
		return m, nil
	}

	// Message pane focused
	if m.activePane == 1 {
		switch {
		case matchKey(msg, keys.Back):
			m.activePane = 0
			return m, nil
		case matchKey(msg, keys.Up):
			m.msgPane.ScrollUp()
			return m, nil
		case matchKey(msg, keys.Down):
			m.msgPane.ScrollDown()
			return m, nil
		}
		return m, nil
	}

	// Conversation list focused
	switch {
	case matchKey(msg, keys.Tab):
		m.convList.CycleTab()
		m.loading = true
		m.msgPane.Clear()
		return m, tea.Batch(m.fetchCmd(), m.spinner.Tick)

	case matchKey(msg, keys.Status):
		m.convList.CycleStatus()
		m.loading = true
		m.msgPane.Clear()
		return m, tea.Batch(m.fetchCmd(), m.spinner.Tick)

	case matchKey(msg, keys.Select):
		sel := m.convList.Selected()
		if sel != nil {
			m.activePane = 1
			return m, fetchMessages(m.client, sel.ID)
		}

	case matchKey(msg, keys.Filter):
		m.convList.StartFilter()
		return m, nil

	case matchKey(msg, keys.Up):
		m.convList.MoveUp()
		m.msgPane.Clear()
		return m, m.fetchContactIfNeeded()
	case matchKey(msg, keys.Down):
		m.convList.MoveDown()
		m.msgPane.Clear()
		return m, m.fetchContactIfNeeded()
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// lipgloss Width() includes padding but excludes border.
	// Visual width = Width + border(2). So Width = m.width - 2.
	barContentW := m.width - 2
	if barContentW < 1 {
		barContentW = 1
	}

	// === Header ===
	host := strings.TrimPrefix(strings.TrimPrefix(m.client.BaseURL, "https://"), "http://")
	leftInfo := fmt.Sprintf("%s  |  Account: %d", host, m.accountID)
	if m.agentName != "" {
		leftInfo += "  |  " + m.agentName
	}

	rightInfo := m.version
	if m.loading {
		rightInfo = m.spinner.View() + " refreshing..."
	}
	if m.err != nil {
		rightInfo = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// barStyle has Padding(0,1) so text area = barContentW - 2
	textW := barContentW - 2
	gap := textW - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
	if gap < 1 {
		gap = 1
	}
	headerContent := leftInfo + strings.Repeat(" ", gap) + rightInfo
	header := barStyle.Width(barContentW).Render(headerContent)

	// === Body columns ===
	convW, msgW, infoW := m.columnWidths()
	bodyH := m.bodyHeight()

	// Conversation list column
	m.convList.SetSize(convW, bodyH)
	convContent := m.convList.View()
	convStyle := columnStyle
	if m.activePane == 0 {
		convStyle = activeColumnStyle
	}
	convCol := convStyle.Width(convW).Height(bodyH).Render(convContent)

	// Messages column
	m.msgPane.SetSize(msgW, bodyH)
	msgContent := m.msgPane.View()
	msgStyle := columnStyle
	if m.activePane == 1 {
		msgStyle = activeColumnStyle
	}
	msgCol := msgStyle.Width(msgW).Height(bodyH).Render(msgContent)

	var body string
	if infoW > 0 {
		infoContent := m.renderInfo(infoW, bodyH)
		infoCol := columnStyle.Width(infoW).Height(bodyH).Render(infoContent)
		body = lipgloss.JoinHorizontal(lipgloss.Top, convCol, msgCol, infoCol)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, convCol, msgCol)
	}

	// === Footer ===
	footer := barStyle.Width(barContentW).Render(helpText(m.convList.Selected() != nil))

	return header + "\n" + body + "\n" + footer
}

func (m Model) renderInfo(w, h int) string {
	conv := m.convList.Selected()
	if conv == nil {
		return lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(w).Height(h).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No conversation selected")
	}

	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	var b strings.Builder

	// Contact section
	b.WriteString(headerStyle.Render("Contact"))
	b.WriteString("\n")
	name, email, phone := "—", "—", "—"
	if conv.Meta.Sender != nil {
		name = conv.Meta.Sender.Name
		if conv.Meta.Sender.Email != "" {
			email = conv.Meta.Sender.Email
		}
		if conv.Meta.Sender.Phone != "" {
			phone = conv.Meta.Sender.Phone
		}
	}
	b.WriteString(labelStyle.Render("Name:  ") + name + "\n")
	b.WriteString(labelStyle.Render("Email: ") + email + "\n")
	b.WriteString(labelStyle.Render("Phone: ") + phone + "\n")

	b.WriteString("\n")

	// Metadata section
	b.WriteString(headerStyle.Render("Metadata"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("ID:       ") + fmt.Sprintf("#%d", conv.ID) + "\n")
	b.WriteString(labelStyle.Render("Status:   ") + statusDot(conv.Status) + " " + conv.Status + "\n")
	b.WriteString(labelStyle.Render("Channel:  ") + conv.Meta.Channel + "\n")
	b.WriteString(labelStyle.Render("Messages: ") + fmt.Sprintf("%d", conv.MessagesCount) + "\n")
	b.WriteString(labelStyle.Render("Created:  ") + formatTime(conv.CreatedAt) + "\n")
	b.WriteString(labelStyle.Render("Activity: ") + formatTime(conv.LastActivityAt) + "\n")

	b.WriteString("\n")

	// Conversation section
	b.WriteString(headerStyle.Render("Conversation"))
	b.WriteString("\n")

	assignee := "—"
	if conv.Meta.Assignee != nil {
		assignee = conv.Meta.Assignee.Name
	}
	b.WriteString(labelStyle.Render("Assignee: ") + assignee + "\n")

	team := "—"
	if conv.Meta.Team != nil {
		team = conv.Meta.Team.Name
	}
	b.WriteString(labelStyle.Render("Team:     ") + team + "\n")

	priority := "—"
	if conv.Priority != nil {
		priority = *conv.Priority
	}
	b.WriteString(labelStyle.Render("Priority: ") + priority + "\n")

	labels := "—"
	if len(conv.Labels) > 0 {
		labels = strings.Join(conv.Labels, ", ")
	}
	b.WriteString(labelStyle.Render("Labels:   ") + labels + "\n")

	// Attributes section
	if len(conv.AdditionalAttributes) > 0 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Attributes"))
		b.WriteString("\n")

		// Browser: "Chrome 120 / macOS" as a single line
		if browser, ok := conv.AdditionalAttributes["browser"].(map[string]interface{}); ok {
			var parts []string
			if name, _ := browser["browser_name"].(string); name != "" {
				ver, _ := browser["browser_version"].(string)
				if ver != "" {
					parts = append(parts, name+" "+ver)
				} else {
					parts = append(parts, name)
				}
			}
			if platform, _ := browser["platform_name"].(string); platform != "" {
				parts = append(parts, platform)
			}
			if len(parts) > 0 {
				b.WriteString(labelStyle.Render("Browser:  ") + strings.Join(parts, " / ") + "\n")
			}
		}

		if lang, ok := conv.AdditionalAttributes["browser_language"].(string); ok && lang != "" {
			b.WriteString(labelStyle.Render("Language: ") + lang + "\n")
		}

		if initiated, ok := conv.AdditionalAttributes["initiated_at"].(map[string]interface{}); ok {
			if ts, _ := initiated["timestamp"].(string); ts != "" {
				b.WriteString(labelStyle.Render("Initiated:") + " " + ts + "\n")
			}
		} else if initiated, ok := conv.AdditionalAttributes["initiated_at"].(string); ok && initiated != "" {
			b.WriteString(labelStyle.Render("Initiated:") + " " + initiated + "\n")
		}
	}

	// Contact attributes (from fetched contact)
	if m.contact != nil && m.contactConvID == conv.ID && len(m.contact.AdditionalAttributes) > 0 {
		attrs := m.contact.AdditionalAttributes
		city, _ := attrs["city"].(string)
		country, _ := attrs["country"].(string)
		if city != "" || country != "" {
			loc := city
			if city != "" && country != "" {
				loc = city + ", " + country
			} else if country != "" {
				loc = country
			}
			b.WriteString(labelStyle.Render("Location: ") + loc + "\n")
		}
		if ip, ok := attrs["created_at_ip"].(string); ok && ip != "" {
			b.WriteString(labelStyle.Render("IP:       ") + ip + "\n")
		}
	}

	return b.String()
}

func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	return key.Matches(msg, binding)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// Run launches the TUI with the given SDK client.
func Run(client *sdk.Client, accountID int, version string) error {
	m := newModel(client, accountID, version)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
