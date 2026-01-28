package tui

import (
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	columnRatio = []int{3, 6, 3}
)

type Model struct {
	client            *sdk.Client
	width             int
	height            int
	focusedColumn     int
	conversationsList *ConversationsList
}

func NewModel(client *sdk.Client) *Model {
	return &Model{
		client:            client,
		width:             80,
		height:            24,
		focusedColumn:     0,
		conversationsList: NewConversationsList(client),
	}
}

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg {
		m.conversationsList.FetchConversations()
		return nil
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.conversationsList.SetSize(m.columnWidth(0), m.height-4)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) View() string {
	colWidths := m.columnWidths()

	left := m.renderColumn(m.conversationsList.View(), colWidths[0], m.focusedColumn == 0)
	center := m.renderColumn(m.centerPlaceholder(), colWidths[1], m.focusedColumn == 1)
	right := m.renderColumn(m.rightPlaceholder(), colWidths[2], m.focusedColumn == 2)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (m *Model) columnWidths() []int {
	totalRatio := 0
	for _, r := range columnRatio {
		totalRatio += r
	}

	widths := make([]int, len(columnRatio))
	for i, r := range columnRatio {
		widths[i] = (m.width * r / totalRatio) - 2
	}
	return widths
}

func (m *Model) columnWidth(col int) int {
	return m.columnWidths()[col]
}

func (m *Model) renderColumn(content string, width int, focused bool) string {
	borderColor := "#7D56F4"
	if focused {
		borderColor = "#FF69B4"
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(width).
		Height(m.height - 4)

	return style.Render(content)
}

func (m *Model) centerPlaceholder() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	return style.Render("Messages\n(coming soon)")
}

func (m *Model) rightPlaceholder() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	return style.Render("Details\n(coming soon)")
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.focusedColumn = (m.focusedColumn + 1) % 3
		return m, nil

	case "shift+tab":
		m.focusedColumn = (m.focusedColumn - 1 + 3) % 3
		return m, nil

	case "up", "k":
		if m.focusedColumn == 0 {
			m.conversationsList.MoveUp()
		}
		return m, nil

	case "down", "j":
		if m.focusedColumn == 0 {
			m.conversationsList.MoveDown()
		}
		return m, nil

	case "r":
		if m.focusedColumn == 0 {
			return m, func() tea.Msg {
				m.conversationsList.FetchConversations()
				return nil
			}
		}
		return m, nil

	case "1":
		if m.focusedColumn == 0 {
			m.conversationsList.ActiveTab = TabMine
			return m, func() tea.Msg {
				m.conversationsList.FetchConversations()
				return nil
			}
		}
		return m, nil

	case "2":
		if m.focusedColumn == 0 {
			m.conversationsList.ActiveTab = TabAll
			return m, func() tea.Msg {
				m.conversationsList.FetchConversations()
				return nil
			}
		}
		return m, nil
	}

	return m, nil
}

// Ensure Model implements tea.Model
var _ tea.Model = (*Model)(nil)
