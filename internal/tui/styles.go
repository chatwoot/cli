package tui

import (
	_ "embed"

	"github.com/charmbracelet/lipgloss"
)

// Pane widths (content width, not including borders)
const (
	convPaneWidth = 40
	infoPaneWidth = 35
)

// Colors — adaptive for light/dark terminals
var (
	colorAccent    = lipgloss.AdaptiveColor{Light: "#1a73e8", Dark: "#8ab4f8"}
	colorMuted     = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	colorBorder    = lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#444444"}
	colorActiveBdr = lipgloss.AdaptiveColor{Light: "#5a9bd5", Dark: "#4a6f8a"}
	colorSelected  = lipgloss.AdaptiveColor{Light: "#e8f0fe", Dark: "#1e3a5f"}
	colorOutgoing  = lipgloss.AdaptiveColor{Light: "#a8c7fa", Dark: "#3d5a80"}
	colorPrivate   = lipgloss.AdaptiveColor{Light: "#b5851e", Dark: "#8a6d3b"}

	colorOpen     = lipgloss.AdaptiveColor{Light: "#0d8043", Dark: "#34a853"}
	colorResolved = lipgloss.AdaptiveColor{Light: "#1967d2", Dark: "#669df6"}
	colorPending  = lipgloss.AdaptiveColor{Light: "#e37400", Dark: "#fbbc04"}
	colorSnoozed  = lipgloss.AdaptiveColor{Light: "#80868b", Dark: "#9aa0a6"}
)

// Header/footer bar style — full-width bordered box
var barStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 1)

// Column styles (for body panes)
var (
	columnStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	activeColumnStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorActiveBdr)
)

// Conversation list styles
var (
	convSelectedStyle = lipgloss.NewStyle().
				Background(colorSelected)

	convSnippetStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	statusTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 1).
			Underline(true)

	statusTabInactive = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)

	filterStyle = lipgloss.NewStyle().
			Foreground(colorAccent)
)

// Status dot
func statusDot(status string) string {
	var color lipgloss.AdaptiveColor
	switch status {
	case "open":
		color = colorOpen
	case "resolved":
		color = colorResolved
	case "pending":
		color = colorPending
	case "snoozed":
		color = colorSnoozed
	default:
		color = colorMuted
	}
	return lipgloss.NewStyle().Foreground(color).Render("●")
}

// Spinner style
var spinnerStyle = lipgloss.NewStyle().Foreground(colorAccent)

// Error style
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))

// Chatwoot logo (speech bubble) — loaded from logo.txt at compile time via embed
//
//go:embed logo.txt
var chatwootLogo string
