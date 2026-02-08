// Package wire provides the communication protocol between UI and Soul.
package wire

import (
	"time"
)

// MessageType represents the type of wire message.
type MessageType string

const (
	// Input messages
	MessageTypeUserInput MessageType = "user_input"
	MessageTypeCancel    MessageType = "cancel"

	// Output messages
	MessageTypeAssistant  MessageType = "assistant"
	MessageTypeToolCall   MessageType = "tool_call"
	MessageTypeToolResult MessageType = "tool_result"
	MessageTypeError      MessageType = "error"
	MessageTypeSystem     MessageType = "system"
	MessageTypeCheckpoint MessageType = "checkpoint"
	MessageTypeClear      MessageType = "clear"

	// Status messages
	MessageTypeStatus   MessageType = "status"
	MessageTypeProgress MessageType = "progress"
	MessageTypeDone     MessageType = "done"
)

// ContentPart represents a part of message content.
type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	JSON     []byte `json:"json,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
}

// Message represents a wire message.
type Message struct {
	Type      MessageType    `json:"type"`
	ID        string         `json:"id,omitempty"`
	ParentID  string         `json:"parent_id,omitempty"`
	Content   []ContentPart  `json:"content,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// NewMessage creates a new wire message.
func NewMessage(msgType MessageType, content ...ContentPart) *Message {
	return &Message{
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewTextMessage creates a new text message.
func NewTextMessage(msgType MessageType, text string) *Message {
	return NewMessage(msgType, ContentPart{
		Type: "text",
		Text: text,
	})
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments []byte `json:"arguments"`
}

// ToolResult represents a tool result.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Success bool   `json:"success"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Checkpoint represents a conversation checkpoint.
type Checkpoint struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
	Context   []byte    `json:"context,omitempty"`
}
