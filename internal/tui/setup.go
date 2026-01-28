package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SetupModel struct {
	inputs     []textinput.Model
	focusIndex int
	done       bool
	config     *config.Config
	err        error
	width      int
	height     int
}

func NewSetupModel() *SetupModel {
	inputs := make([]textinput.Model, 3)

	// URL input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "https://app.chatwoot.com"
	inputs[0].Focus()
	inputs[0].CharLimit = 256
	inputs[0].Width = 50

	// API Key input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "your-api-access-token"
	inputs[1].CharLimit = 256
	inputs[1].Width = 50
	inputs[1].EchoMode = textinput.EchoPassword
	inputs[1].EchoCharacter = '•'

	// Account ID input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "1"
	inputs[2].CharLimit = 20
	inputs[2].Width = 50

	return &SetupModel{
		inputs:     inputs,
		focusIndex: 0,
	}
}

func (m *SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "down", "enter":
			if msg.String() == "enter" && m.focusIndex == len(m.inputs) {
				return m.submit()
			}

			// Move focus to next input
			m.focusIndex = (m.focusIndex + 1) % (len(m.inputs) + 1)

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)

		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex - 1 + len(m.inputs) + 1) % (len(m.inputs) + 1)

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	// Handle input updates
	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *SetupModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *SetupModel) submit() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(m.inputs[0].Value())
	apiKey := strings.TrimSpace(m.inputs[1].Value())
	accountIDStr := strings.TrimSpace(m.inputs[2].Value())

	if url == "" || apiKey == "" || accountIDStr == "" {
		m.err = fmt.Errorf("all fields are required")
		return m, nil
	}

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		m.err = fmt.Errorf("invalid account ID: must be a number")
		return m, nil
	}

	cfg := &config.Config{
		BaseURL:   url,
		APIKey:    apiKey,
		AccountID: accountID,
	}

	if err := config.Save(cfg); err != nil {
		m.err = fmt.Errorf("failed to save config: %w", err)
		return m, nil
	}

	m.config = cfg
	m.done = true
	return m, tea.Quit
}

func (m *SetupModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	focusedLabelStyle := labelStyle.
		Foreground(lipgloss.Color("#FF69B4"))

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#626262")).
		Padding(0, 3).
		MarginTop(1)

	focusedButtonStyle := buttonStyle.
		Background(lipgloss.Color("#7D56F4"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF4444")).
		MarginTop(1)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(2)

	labels := []string{"Chatwoot URL", "API Key", "Account ID"}

	var b strings.Builder

	b.WriteString(titleStyle.Render(" Chatwoot TUI Setup "))
	b.WriteString("\n\n")

	for i, input := range m.inputs {
		lStyle := labelStyle
		if i == m.focusIndex {
			lStyle = focusedLabelStyle
		}
		b.WriteString(lStyle.Render(labels[i]))
		b.WriteString("\n")
		b.WriteString(input.View())
		b.WriteString("\n\n")
	}

	// Submit button
	btnStyle := buttonStyle
	if m.focusIndex == len(m.inputs) {
		btnStyle = focusedButtonStyle
	}
	b.WriteString(btnStyle.Render("Save & Continue"))

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Tab/↓ next • Shift+Tab/↑ prev • Enter submit • Esc quit"))

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *SetupModel) Done() bool {
	return m.done
}

func (m *SetupModel) Config() *config.Config {
	return m.config
}
