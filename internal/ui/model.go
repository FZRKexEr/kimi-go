package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"kimi-go/internal/soul"
	"kimi-go/internal/wire"
)

// Model is the bubbletea model for the TUI.
type Model struct {
	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	messages []chatMsg
	loading  bool
	ready    bool
	quitting bool

	soul       *soul.Soul
	eventCh    <-chan tea.Msg
	mdRenderer *markdownRenderer
	width      int
	height     int

	// Streaming state
	streaming      bool
	streamingIndex int // Index of the message being streamed
}

// NewModel creates a new TUI model.
func NewModel(s *soul.Soul, eventCh <-chan tea.Msg) Model {
	// Textarea setup
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 3
	ta.SetHeight(1)
	ta.Prompt = "│ "
	// Enter sends message (intercepted in Update), Alt+Enter inserts newline
	ta.KeyMap.InsertNewline.SetKeys("alt+enter", "shift+enter")

	// Spinner setup
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return Model{
		textarea:   ta,
		spinner:    sp,
		soul:       s,
		eventCh:    eventCh,
		mdRenderer: newMarkdownRenderer(80),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		waitForSoulEvent(m.eventCh),
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Rebuild markdown renderer for new width
		m.mdRenderer = newMarkdownRenderer(msg.Width)

		// Layout: header(1) + viewport + divider(1) + input(3) + help(1)
		headerHeight := 1
		footerHeight := 5 // divider + input area + help
		vpHeight := msg.Height - headerHeight - footerHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.viewport.MouseWheelEnabled = true
			m.viewport.MouseWheelDelta = 3
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}

		// Update textarea width
		m.textarea.SetWidth(msg.Width - 2)

		// Re-render conversation for new width
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading {
				// Ignore enter while loading
				return m, nil
			}
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				return m, nil
			}
			if text == "exit" || text == "quit" {
				m.quitting = true
				return m, tea.Quit
			}
			// Add user message to display
			m.messages = append(m.messages, chatMsg{
				Role:    string(wire.MessageTypeUserInput),
				Content: text,
			})
			m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
			m.viewport.GotoBottom()

			// Clear input
			m.textarea.Reset()
			m.loading = true
			m.textarea.Blur()

			// Send to Soul asynchronously
			cmds = append(cmds, sendToSoul(m.soul, text))
			return m, tea.Batch(cmds...)
		}

	case SoulMessageMsg:
		// Check if this is a streaming update (assistant message with existing content)
		newMsg := newChatMsgFromWire(msg.Message)
		if newMsg.Role == string(wire.MessageTypeAssistant) && m.streaming && m.streamingIndex >= 0 {
			// Update existing streaming message
			m.messages[m.streamingIndex] = newMsg
		} else {
			// New message
			m.messages = append(m.messages, newMsg)
			// If it's an assistant message and we're loading, mark as streaming
			if newMsg.Role == string(wire.MessageTypeAssistant) && m.loading {
				m.streaming = true
				m.streamingIndex = len(m.messages) - 1
			}
		}
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()
		cmds = append(cmds, waitForSoulEvent(m.eventCh))

	case SoulToolCallMsg:
		m.messages = append(m.messages, chatMsg{
			Role:    string(wire.MessageTypeToolCall),
			Content: fmt.Sprintf("Calling %s: %s", msg.ToolCall.Name, string(msg.ToolCall.Arguments)),
		})
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()
		cmds = append(cmds, waitForSoulEvent(m.eventCh))

	case SoulToolResultMsg:
		content := msg.ToolResult.Result
		if !msg.ToolResult.Success {
			content = "Error: " + msg.ToolResult.Error
		}
		m.messages = append(m.messages, chatMsg{
			Role:    string(wire.MessageTypeToolResult),
			Content: content,
		})
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()
		cmds = append(cmds, waitForSoulEvent(m.eventCh))

	case SoulErrorMsg:
		m.messages = append(m.messages, chatMsg{
			Role:    string(wire.MessageTypeError),
			Content: msg.Err.Error(),
		})
		m.loading = false
		m.streaming = false
		m.streamingIndex = -1
		m.textarea.Focus()
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()
		cmds = append(cmds, waitForSoulEvent(m.eventCh))

	case SoulDoneMsg:
		m.loading = false
		m.streaming = false
		m.streamingIndex = -1
		m.textarea.Focus()
		cmds = append(cmds, waitForSoulEvent(m.eventCh))

	case errMsg:
		m.messages = append(m.messages, chatMsg{
			Role:    string(wire.MessageTypeError),
			Content: msg.err.Error(),
		})
		m.loading = false
		m.streaming = false
		m.streamingIndex = -1
		m.textarea.Focus()
		m.viewport.SetContent(renderConversation(m.messages, m.mdRenderer))
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update textarea (for non-enter keys)
	if !m.loading {
		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
	}

	// Update viewport (for scrolling)
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	if !m.ready {
		return "\n  Initializing..."
	}

	// Header
	header := appTitleStyle.Render(" Kimi-Go ")

	// Divider
	divider := dividerStyle.Render(strings.Repeat("─", m.width))

	// Input area or spinner
	var inputArea string
	if m.loading {
		if m.streaming {
			inputArea = fmt.Sprintf("  %s Receiving...", m.spinner.View())
		} else {
			inputArea = fmt.Sprintf("  %s Thinking...", m.spinner.View())
		}
	} else {
		inputArea = m.textarea.View()
	}

	// Footer help
	footer := helpStyle.Render("  Enter: send | Alt+Enter: newline | Ctrl+C: quit")

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s",
		header,
		m.viewport.View(),
		divider,
		inputArea,
		footer,
	)
}

// waitForSoulEvent returns a command that waits for the next Soul event.
func waitForSoulEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return SoulDoneMsg{}
		}
		return msg
	}
}

// sendToSoul sends a user message to Soul asynchronously.
func sendToSoul(s *soul.Soul, text string) tea.Cmd {
	return func() tea.Msg {
		msg := wire.NewTextMessage(wire.MessageTypeUserInput, text)
		if err := s.SendMessage(*msg); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}
