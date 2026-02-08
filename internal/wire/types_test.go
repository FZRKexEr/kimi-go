// Package wire provides the communication protocol between UI and Soul.
package wire

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessageTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value MessageType
		want  string
	}{
		{"UserInput", MessageTypeUserInput, "user_input"},
		{"Cancel", MessageTypeCancel, "cancel"},
		{"Assistant", MessageTypeAssistant, "assistant"},
		{"ToolCall", MessageTypeToolCall, "tool_call"},
		{"ToolResult", MessageTypeToolResult, "tool_result"},
		{"Error", MessageTypeError, "error"},
		{"System", MessageTypeSystem, "system"},
		{"Checkpoint", MessageTypeCheckpoint, "checkpoint"},
		{"Clear", MessageTypeClear, "clear"},
		{"Status", MessageTypeStatus, "status"},
		{"Progress", MessageTypeProgress, "progress"},
		{"Done", MessageTypeDone, "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, tt.value)
			}
		})
	}
}

func TestNewMessage(t *testing.T) {
	content := []ContentPart{
		{Type: "text", Text: "Hello"},
	}
	msg := NewMessage(MessageTypeUserInput, content...)

	if msg.Type != MessageTypeUserInput {
		t.Errorf("Expected type %s, got %s", MessageTypeUserInput, msg.Type)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content part, got %d", len(msg.Content))
	}

	if msg.Content[0].Text != "Hello" {
		t.Errorf("Expected text 'Hello', got %s", msg.Content[0].Text)
	}

	if msg.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage(MessageTypeAssistant, "Hello, World!")

	if msg.Type != MessageTypeAssistant {
		t.Errorf("Expected type %s, got %s", MessageTypeAssistant, msg.Type)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content part, got %d", len(msg.Content))
	}

	if msg.Content[0].Type != "text" {
		t.Errorf("Expected type 'text', got %s", msg.Content[0].Type)
	}

	if msg.Content[0].Text != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got %s", msg.Content[0].Text)
	}
}

func TestMessageJSONSerialization(t *testing.T) {
	content := []ContentPart{
		{Type: "text", Text: "Hello"},
	}
	msg := NewMessage(MessageTypeUserInput, content...)

	// Serialize to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Deserialize from JSON
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Verify fields
	if decoded.Type != msg.Type {
		t.Errorf("Expected type %s, got %s", msg.Type, decoded.Type)
	}

	if len(decoded.Content) != len(msg.Content) {
		t.Errorf("Expected %d content parts, got %d", len(msg.Content), len(decoded.Content))
	}
}

func TestToolCall(t *testing.T) {
	call := ToolCall{
		ID:        "call-123",
		Name:      "shell",
		Arguments: []byte(`{"command": "ls -la"}`),
	}

	if call.ID != "call-123" {
		t.Errorf("Expected ID 'call-123', got %s", call.ID)
	}

	if call.Name != "shell" {
		t.Errorf("Expected name 'shell', got %s", call.Name)
	}

	expectedArgs := `{"command": "ls -la"}`
	if string(call.Arguments) != expectedArgs {
		t.Errorf("Expected arguments %s, got %s", expectedArgs, string(call.Arguments))
	}
}

func TestToolResult(t *testing.T) {
	result := ToolResult{
		CallID:  "call-123",
		Success: true,
		Result:  "output of command",
	}

	if result.CallID != "call-123" {
		t.Errorf("Expected CallID 'call-123', got %s", result.CallID)
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Result != "output of command" {
		t.Errorf("Expected Result 'output of command', got %s", result.Result)
	}
}

func TestToolResult_Error(t *testing.T) {
	result := ToolResult{
		CallID:  "call-456",
		Success: false,
		Error:   "command not found",
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error != "command not found" {
		t.Errorf("Expected Error 'command not found', got %s", result.Error)
	}
}

func TestCheckpoint(t *testing.T) {
	checkpoint := Checkpoint{
		ID:        "checkpoint-123",
		MessageID: "msg-456",
		Timestamp: time.Now(),
		Context:   []byte(`{"key": "value"}`),
	}

	if checkpoint.ID != "checkpoint-123" {
		t.Errorf("Expected ID 'checkpoint-123', got %s", checkpoint.ID)
	}

	if checkpoint.MessageID != "msg-456" {
		t.Errorf("Expected MessageID 'msg-456', got %s", checkpoint.MessageID)
	}

	if checkpoint.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	expectedContext := `{"key": "value"}`
	if string(checkpoint.Context) != expectedContext {
		t.Errorf("Expected Context %s, got %s", expectedContext, string(checkpoint.Context))
	}
}
