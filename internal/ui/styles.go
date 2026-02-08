package ui

import "github.com/charmbracelet/lipgloss"

var (
	// userStyle styles user messages (cyan bold).
	userStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	// assistantLabelStyle styles the "Assistant:" label (green bold).
	assistantLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	// toolCallStyle styles tool call messages (yellow).
	toolCallStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// toolResultBorderStyle styles tool result blocks (gray rounded border).
	toolResultBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(0, 1)

	// errorStyle styles error messages (red bold).
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

	// spinnerStyle styles the spinner (magenta).
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))

	// helpStyle styles the bottom help text (dark gray).
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// appTitleStyle styles the app title (blue bold).
	appTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)

	// dividerStyle styles the divider line (gray).
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// approvalWarningStyle styles the approval warning for dangerous tools (red bold).
	approvalWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Bold(true).
				Background(lipgloss.Color("0"))

	// approvalSafeStyle styles the approval prompt for safe tools (yellow).
	approvalSafeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true)

	// keyStyle styles key press hints (cyan bold).
	keyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)
)
