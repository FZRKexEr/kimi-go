// Package tools provides tool implementations for kimi-go.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool represents a tool that can be called by the agent.
type Tool interface {
	// Name returns the tool name.
	Name() string
	// Description returns the tool description.
	Description() string
	// Parameters returns the JSON schema for tool parameters.
	Parameters() json.RawMessage
	// Execute executes the tool with the given arguments.
	Execute(ctx context.Context, args json.RawMessage) (any, error)
}

// ToolSet manages a collection of tools.
type ToolSet struct {
	tools map[string]Tool
}

// NewToolSet creates a new tool set.
func NewToolSet() *ToolSet {
	return &ToolSet{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool.
func (ts *ToolSet) Register(tool Tool) error {
	name := tool.Name()
	if _, exists := ts.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	ts.tools[name] = tool
	return nil
}

// Get gets a tool by name.
func (ts *ToolSet) Get(name string) (Tool, error) {
	tool, exists := ts.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return tool, nil
}

// List lists all registered tools.
func (ts *ToolSet) List() []Tool {
	result := make([]Tool, 0, len(ts.tools))
	for _, tool := range ts.tools {
		result = append(result, tool)
	}
	return result
}

// Execute executes a tool by name.
func (ts *ToolSet) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	tool, err := ts.Get(name)
	if err != nil {
		return nil, err
	}
	return tool.Execute(ctx, args)
}

// ToolInfo represents information about a tool for LLM.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents a tool result.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Success bool   `json:"success"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// TruncateLimits defines the truncation limits for tool output.
const (
	MaxToolOutputTotalChars = 50000 // Maximum total characters for tool output
	MaxToolOutputLineChars  = 2000  // Maximum characters per line
)

// TruncateForLLM truncates the tool result for sending to LLM.
// It applies per-line and total character limits.
func (tr *ToolResult) TruncateForLLM() *ToolResult {
	if tr == nil {
		return nil
	}

	// Create a copy to avoid modifying the original (for UI display)
	truncated := &ToolResult{
		CallID:  tr.CallID,
		Success: tr.Success,
		Result:  tr.Result,
		Error:   tr.Error,
	}

	// Truncate the result field
	if len(truncated.Result) > 0 {
		truncated.Result = truncateText(truncated.Result, MaxToolOutputTotalChars, MaxToolOutputLineChars)
	}

	// Also truncate error if present (though typically errors are short)
	if len(truncated.Error) > MaxToolOutputTotalChars {
		truncated.Error = truncateText(truncated.Error, MaxToolOutputTotalChars, MaxToolOutputLineChars)
	}

	return truncated
}

// truncateText applies line-by-line and total character limits.
func truncateText(text string, maxTotal, maxLine int) string {
	if len(text) <= maxTotal && maxLine <= 0 {
		return text
	}

	var result []byte
	totalLen := 0
	lineStart := 0
	truncated := false

	for i := 0; i <= len(text); i++ {
		// Check for line end or end of text
		if i == len(text) || text[i] == '\n' {
			line := text[lineStart:i]
			lineEnd := i
			if i < len(text) {
				lineEnd++ // Include the newline
			}

			// Check if adding this line would exceed total limit
			lineLen := len(line)
			if lineLen > maxLine {
				// Truncate this line
				lineLen = maxLine
				truncated = true
			}

			if totalLen+lineLen > maxTotal {
				// Can't fit the whole line, add what we can
				remaining := maxTotal - totalLen
				if remaining > 0 {
					result = append(result, line[:remaining]...)
					totalLen += remaining
				}
				truncated = true
				break
			}

			// Add the (possibly truncated) line
			if len(line) > maxLine {
				result = append(result, line[:maxLine]...)
				if i < len(text) {
					result = append(result, '\n')
				}
			} else {
				result = append(result, text[lineStart:lineEnd]...)
			}
			totalLen += lineLen
			if i < len(text) && len(line) <= maxLine {
				totalLen++ // for newline
			}

			lineStart = i + 1
		}
	}

	if truncated {
		result = append(result, "\n... (output truncated)"...)
	}

	return string(result)
}

// GetToolInfo returns tool information for all registered tools.
func (ts *ToolSet) GetToolInfo() []ToolInfo {
	tools := ts.List()
	result := make([]ToolInfo, len(tools))
	for i, tool := range tools {
		result[i] = ToolInfo{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		}
	}
	return result
}
