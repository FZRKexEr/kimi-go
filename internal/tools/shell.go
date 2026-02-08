package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ShellTool executes shell commands.
type ShellTool struct {
	workDir string
	timeout time.Duration
}

// ShellToolParams represents parameters for shell tool.
type ShellToolParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// ShellToolResult represents the result of a shell execution.
type ShellToolResult struct {
	Success  bool   `json:"success"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// NewShellTool creates a new shell tool.
func NewShellTool(workDir string, timeout time.Duration) *ShellTool {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &ShellTool{
		workDir: workDir,
		timeout: timeout,
	}
}

// Name returns the tool name.
func (t *ShellTool) Name() string {
	return "shell"
}

// Description returns the tool description.
func (t *ShellTool) Description() string {
	return "Execute shell commands. Use this tool to run commands in the shell."
}

// Parameters returns the JSON schema for tool parameters.
func (t *ShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in seconds (default: 60)",
				"minimum": 1,
				"maximum": 300
			}
		},
		"required": ["command"]
	}`)
}

// Execute executes the shell tool.
func (t *ShellTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var params ShellToolParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if strings.TrimSpace(params.Command) == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}

	output, err := cmd.CombinedOutput()

	result := ShellToolResult{
		Success:  err == nil,
		Stdout:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Error = "command timed out"
			result.ExitCode = -1
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

// SetWorkDir sets the working directory for shell commands.
func (t *ShellTool) SetWorkDir(dir string) {
	t.workDir = dir
}
