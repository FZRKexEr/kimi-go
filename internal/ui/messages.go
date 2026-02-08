// Package ui provides the terminal UI for kimi-go using Charm libraries.
package ui

import (
	"kimi-go/internal/approval"
	"kimi-go/internal/soul"
	"kimi-go/internal/tools"
	"kimi-go/internal/wire"
)

// SoulMessageMsg carries a wire.Message from Soul's OnMessage callback.
type SoulMessageMsg struct {
	Message wire.Message
}

// SoulToolCallMsg carries a tools.ToolCall from Soul's OnToolCall callback.
type SoulToolCallMsg struct {
	ToolCall tools.ToolCall
}

// SoulToolResultMsg carries a tools.ToolResult from Soul's OnToolResult callback.
type SoulToolResultMsg struct {
	ToolResult tools.ToolResult
}

// SoulErrorMsg carries an error from Soul's OnError callback.
type SoulErrorMsg struct {
	Err error
}

// SoulDoneMsg signals that Soul has finished processing a message.
type SoulDoneMsg struct{}

// ApprovalRequestMsg signals that user approval is needed for a tool call.
type ApprovalRequestMsg struct {
	Request *approval.ApprovalRequest
	Soul    *soul.Soul
}

// errMsg is an internal UI error.
type errMsg struct {
	err error
}
