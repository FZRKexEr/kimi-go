package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/glamour"

	"kimi-go/internal/wire"
)

// chatMsg represents a single message in the conversation display.
type chatMsg struct {
	Role    string
	Content string
	Time    time.Time
}

// newChatMsgFromWire converts a wire.Message to a chatMsg.
func newChatMsgFromWire(msg wire.Message) chatMsg {
	var content string
	for _, part := range msg.Content {
		if part.Type == "text" {
			content = part.Text
			break
		}
	}
	return chatMsg{
		Role:    string(msg.Type),
		Content: content,
		Time:    msg.Timestamp,
	}
}

// markdownRenderer wraps glamour for terminal markdown rendering.
type markdownRenderer struct {
	renderer *glamour.TermRenderer
	width    int
}

// newMarkdownRenderer creates a new markdown renderer for the given width.
func newMarkdownRenderer(width int) *markdownRenderer {
	if width <= 0 {
		width = 80
	}
	// Leave some margin for borders and padding
	renderWidth := width - 4
	if renderWidth < 40 {
		renderWidth = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(renderWidth),
	)
	if err != nil {
		// Fallback: return a renderer that just passes through text
		return &markdownRenderer{width: width}
	}
	return &markdownRenderer{renderer: r, width: width}
}

// render renders markdown text to styled terminal output.
func (m *markdownRenderer) render(text string) string {
	if m.renderer == nil {
		return text
	}
	out, err := m.renderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

const maxToolResultLen = 2000

// renderMessage renders a single chatMsg to styled terminal output.
func renderMessage(msg chatMsg, md *markdownRenderer) string {
	switch msg.Role {
	case string(wire.MessageTypeUserInput):
		return userStyle.Render("> " + msg.Content)

	case string(wire.MessageTypeAssistant):
		label := assistantLabelStyle.Render("Assistant:")
		body := md.render(msg.Content)
		return label + "\n" + body

	case string(wire.MessageTypeToolCall):
		return toolCallStyle.Render(">> " + msg.Content)

	case string(wire.MessageTypeToolResult):
		content := msg.Content
		if len(content) > maxToolResultLen {
			content = content[:maxToolResultLen] + "\n... (truncated)"
		}
		return toolResultBorderStyle.Render(content)

	case string(wire.MessageTypeError):
		return errorStyle.Render("Error: " + msg.Content)

	default:
		return msg.Content
	}
}

// renderConversation renders all messages into a single string.
func renderConversation(msgs []chatMsg, md *markdownRenderer) string {
	if len(msgs) == 0 {
		return helpStyle.Render("  Type a message and press Enter to start chatting.")
	}
	var b strings.Builder
	for i, msg := range msgs {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderMessage(msg, md))
	}
	return b.String()
}
