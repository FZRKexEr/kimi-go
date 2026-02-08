// Package tools provides tool implementations for kimi-go.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// MockTool is a mock implementation of Tool for testing.
type MockTool struct {
	name        string
	description string
	params      json.RawMessage
	executeFunc func(ctx context.Context, args json.RawMessage) (any, error)
}

func (m *MockTool) Name() string                { return m.name }
func (m *MockTool) Description() string         { return m.description }
func (m *MockTool) Parameters() json.RawMessage { return m.params }
func (m *MockTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, args)
	}
	return nil, nil
}

func TestNewToolSet(t *testing.T) {
	ts := NewToolSet()
	if ts == nil {
		t.Fatal("Expected non-nil ToolSet")
	}

	if len(ts.List()) != 0 {
		t.Errorf("Expected empty toolset, got %d tools", len(ts.List()))
	}
}

func TestToolSet_Register(t *testing.T) {
	ts := NewToolSet()

	mock := &MockTool{name: "test-tool"}

	if err := ts.Register(mock); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(ts.List()) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(ts.List()))
	}
}

func TestToolSet_Register_Duplicate(t *testing.T) {
	ts := NewToolSet()

	mock1 := &MockTool{name: "test-tool"}
	mock2 := &MockTool{name: "test-tool"}

	if err := ts.Register(mock1); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err := ts.Register(mock2)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestToolSet_Get(t *testing.T) {
	ts := NewToolSet()

	mock := &MockTool{name: "test-tool"}
	if err := ts.Register(mock); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tool, err := ts.Get("test-tool")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if tool.Name() != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", tool.Name())
	}
}

func TestToolSet_Get_NotFound(t *testing.T) {
	ts := NewToolSet()

	_, err := ts.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
}

func TestToolSet_List(t *testing.T) {
	ts := NewToolSet()

	// Empty list
	tools := ts.List()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(tools))
	}

	// Add tools
	mock1 := &MockTool{name: "tool-1"}
	mock2 := &MockTool{name: "tool-2"}

	if err := ts.Register(mock1); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := ts.Register(mock2); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tools = ts.List()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestToolSet_Execute(t *testing.T) {
	ts := NewToolSet()

	executed := false
	mock := &MockTool{
		name: "test-tool",
		executeFunc: func(ctx context.Context, args json.RawMessage) (any, error) {
			executed = true
			return "result", nil
		},
	}

	if err := ts.Register(mock); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result, err := ts.Execute(context.Background(), "test-tool", []byte(`{}`))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !executed {
		t.Error("Tool was not executed")
	}

	if result != "result" {
		t.Errorf("Expected result 'result', got %v", result)
	}
}

func TestToolSet_Execute_NotFound(t *testing.T) {
	ts := NewToolSet()

	_, err := ts.Execute(context.Background(), "nonexistent", []byte(`{}`))
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
}

func TestToolSet_GetToolInfo(t *testing.T) {
	ts := NewToolSet()

	mock := &MockTool{
		name:        "test-tool",
		description: "Test tool description",
		params:      []byte(`{"type": "object"}`),
	}

	if err := ts.Register(mock); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	infos := ts.GetToolInfo()
	if len(infos) != 1 {
		t.Fatalf("Expected 1 tool info, got %d", len(infos))
	}

	if infos[0].Name != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", infos[0].Name)
	}

	if infos[0].Description != "Test tool description" {
		t.Errorf("Expected description 'Test tool description', got %s", infos[0].Description)
	}
}

func TestToolCall(t *testing.T) {
	call := ToolCall{
		ID:        "call-123",
		Name:      "test-tool",
		Arguments: []byte(`{"key": "value"}`),
	}

	if call.ID != "call-123" {
		t.Errorf("Expected ID 'call-123', got %s", call.ID)
	}

	if call.Name != "test-tool" {
		t.Errorf("Expected Name 'test-tool', got %s", call.Name)
	}

	expectedArgs := `{"key": "value"}`
	if string(call.Arguments) != expectedArgs {
		t.Errorf("Expected Arguments %s, got %s", expectedArgs, string(call.Arguments))
	}
}

func TestToolResult(t *testing.T) {
	result := ToolResult{
		CallID:  "call-123",
		Success: true,
		Result:  "success output",
		Error:   "",
	}

	if result.CallID != "call-123" {
		t.Errorf("Expected CallID 'call-123', got %s", result.CallID)
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Result != "success output" {
		t.Errorf("Expected Result 'success output', got %s", result.Result)
	}

	// Test error result
	errorResult := ToolResult{
		CallID:  "call-456",
		Success: false,
		Error:   "something went wrong",
	}

	if errorResult.Success {
		t.Error("Expected Success to be false")
	}

	if errorResult.Error != "something went wrong" {
		t.Errorf("Expected Error 'something went wrong', got %s", errorResult.Error)
	}
}

func TestToolInfo(t *testing.T) {
	info := ToolInfo{
		Name:        "test-tool",
		Description: "A test tool",
		Parameters:  []byte(`{"type": "object"}`),
	}

	if info.Name != "test-tool" {
		t.Errorf("Expected Name 'test-tool', got %s", info.Name)
	}

	if info.Description != "A test tool" {
		t.Errorf("Expected Description 'A test tool', got %s", info.Description)
	}

	expectedParams := `{"type": "object"}`
	if string(info.Parameters) != expectedParams {
		t.Errorf("Expected Parameters %s, got %s", expectedParams, string(info.Parameters))
	}
}

func TestContextWithTimeout(t *testing.T) {
	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(10 * time.Millisecond)

	// Check if context is done
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Expected context to be done")
	}

	// Check error
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", ctx.Err())
	}
}

func TestContextWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context
	cancel()

	// Check if context is done
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Expected context to be done")
	}

	// Check error
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected Canceled, got %v", ctx.Err())
	}
}

func TestTruncateForLLM(t *testing.T) {
	tests := []struct {
		name          string
		result        *ToolResult
		wantTruncated bool
	}{
		{
			name: "short result - no truncation",
			result: &ToolResult{
				CallID:  "call-1",
				Success: true,
				Result:  "short result",
			},
			wantTruncated: false,
		},
		{
			name: "long result - truncated",
			result: &ToolResult{
				CallID:  "call-2",
				Success: true,
				Result:  strings.Repeat("a", MaxToolOutputTotalChars+100),
			},
			wantTruncated: true,
		},
		{
			name: "error result - truncated",
			result: &ToolResult{
				CallID:  "call-3",
				Success: false,
				Error:   strings.Repeat("e", MaxToolOutputTotalChars+100),
			},
			wantTruncated: true,
		},
		{
			name:          "nil result",
			result:        nil,
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncated := tt.result.TruncateForLLM()
			if tt.result == nil {
				if truncated != nil {
					t.Errorf("TruncateForLLM() = %v, want nil", truncated)
				}
				return
			}

			if tt.result.Success {
				hasTruncatedMsg := strings.Contains(truncated.Result, "... (output truncated)")
				if hasTruncatedMsg != tt.wantTruncated {
					t.Errorf("TruncateForLLM() truncated = %v, want %v", hasTruncatedMsg, tt.wantTruncated)
				}
			} else {
				hasTruncatedMsg := strings.Contains(truncated.Error, "... (output truncated)")
				if hasTruncatedMsg != tt.wantTruncated {
					t.Errorf("TruncateForLLM() error truncated = %v, want %v", hasTruncatedMsg, tt.wantTruncated)
				}
			}

			// Verify original is not modified
			if len(tt.result.Result) > MaxToolOutputTotalChars+100 {
				if len(tt.result.Result) != MaxToolOutputTotalChars+100 {
					t.Error("Original result was modified")
				}
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxTotal int
		maxLine  int
		want     string
	}{
		{
			name:     "no truncation needed",
			text:     "short text",
			maxTotal: 100,
			maxLine:  50,
			want:     "short text",
		},
		{
			name:     "long line truncated",
			text:     strings.Repeat("a", 3000),
			maxTotal: 50000,
			maxLine:  2000,
			want:     strings.Repeat("a", 2000) + "\n... (output truncated)",
		},
		{
			name:     "empty string",
			text:     "",
			maxTotal: 100,
			maxLine:  50,
			want:     "",
		},
		{
			name:     "exactly at limit",
			text:     strings.Repeat("a", 100),
			maxTotal: 100,
			maxLine:  2000,
			want:     strings.Repeat("a", 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateText(tt.text, tt.maxTotal, tt.maxLine)
			if got != tt.want {
				t.Errorf("truncateText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateTextPerLine(t *testing.T) {
	// Test that long lines are truncated
	longLine := strings.Repeat("a", 2500)
	text := "short line\n" + longLine + "\nanother short line"

	result := truncateText(text, 50000, 2000)

	// Check that the long line was truncated
	if !strings.Contains(result, strings.Repeat("a", 2000)) {
		t.Error("Long line should be truncated to 2000 chars")
	}

	// Check that short lines are preserved
	if !strings.Contains(result, "short line") {
		t.Error("Short lines should be preserved")
	}

	// Check truncation message is present
	if !strings.Contains(result, "... (output truncated)") {
		t.Error("Truncation message should be present")
	}
}

func TestTruncateTextTotalLimit(t *testing.T) {
	// Create text that exceeds total limit
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 1000)
	}
	text := strings.Join(lines, "\n")

	result := truncateText(text, 50000, 2000)

	// Result should be at most 50000 + truncation message
	if len(result) > 50000+len("\n... (output truncated)") {
		t.Errorf("Result length %d exceeds limit", len(result))
	}

	// Should have truncation message
	if !strings.HasSuffix(result, "... (output truncated)") {
		t.Error("Result should end with truncation message")
	}
}
