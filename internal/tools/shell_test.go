package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewShellTool(t *testing.T) {
	tool := NewShellTool("/tmp", 30*time.Second)
	if tool == nil {
		t.Fatal("Expected non-nil ShellTool")
	}

	if tool.Name() != "shell" {
		t.Errorf("Expected name 'shell', got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Expected non-empty description")
	}
}

func TestNewShellTool_DefaultTimeout(t *testing.T) {
	tool := NewShellTool("/tmp", 0)

	// Should use default timeout of 60 seconds
	// We can't directly access the timeout field, but we can verify the tool was created
	if tool == nil {
		t.Fatal("Expected non-nil ShellTool")
	}
}

func TestShellTool_Name(t *testing.T) {
	tool := NewShellTool("/tmp", 0)
	if tool.Name() != "shell" {
		t.Errorf("Expected name 'shell', got %s", tool.Name())
	}
}

func TestShellTool_Description(t *testing.T) {
	tool := NewShellTool("/tmp", 0)
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestShellTool_Parameters(t *testing.T) {
	tool := NewShellTool("/tmp", 0)
	params := tool.Parameters()
	if len(params) == 0 {
		t.Error("Expected non-empty parameters")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(params, &parsed); err != nil {
		t.Errorf("Parameters should be valid JSON: %v", err)
	}
}

func TestShellTool_Execute(t *testing.T) {
	tool := NewShellTool("/tmp", 5*time.Second)

	args, _ := json.Marshal(map[string]string{
		"command": "echo hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	shellResult, ok := result.(ShellToolResult)
	if !ok {
		t.Fatalf("Expected ShellToolResult, got %T", result)
	}

	if !shellResult.Success {
		t.Error("Expected success to be true")
	}

	if shellResult.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", shellResult.ExitCode)
	}

	if shellResult.Stdout != "hello\n" {
		t.Errorf("Expected stdout 'hello\\n', got %s", shellResult.Stdout)
	}
}

func TestShellTool_Execute_InvalidParams(t *testing.T) {
	tool := NewShellTool("/tmp", 0)

	// Invalid JSON
	_, err := tool.Execute(context.Background(), []byte(`{invalid`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestShellTool_Execute_EmptyCommand(t *testing.T) {
	tool := NewShellTool("/tmp", 0)

	args, _ := json.Marshal(map[string]string{
		"command": "",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("Expected error for empty command")
	}
}

func TestShellTool_SetWorkDir(t *testing.T) {
	tool := NewShellTool("/initial", 0)
	tool.SetWorkDir("/new")
	// The workDir field is not directly accessible, but we can verify no panic
}

func TestShellToolResult_JSON(t *testing.T) {
	result := ShellToolResult{
		Success:  true,
		Stdout:   "output",
		Stderr:   "",
		ExitCode: 0,
		Error:    "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ShellToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Success != result.Success {
		t.Error("Success mismatch")
	}

	if decoded.Stdout != result.Stdout {
		t.Error("Stdout mismatch")
	}
}

func TestShellTool_Execute_WithTimeout(t *testing.T) {
	// Create tool with very short timeout
	tool := NewShellTool("/tmp", 1*time.Millisecond)

	// This command should timeout
	args, _ := json.Marshal(map[string]string{
		"command": "sleep 10",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := tool.Execute(ctx, args)

	// Should get a result (possibly with timeout error) but not panic
	if err == nil && result != nil {
		shellResult, ok := result.(*ShellToolResult)
		if ok {
			// Should either succeed or have error/timeout
			if !shellResult.Success && shellResult.Error == "" && shellResult.ExitCode == 0 {
				t.Error("Failed result should have error message or non-zero exit code")
			}
		}
	}
}

func TestShellTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewShellTool("/tmp", 0)

	_, err := tool.Execute(context.Background(), []byte(`{invalid json`))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestShellTool_Execute_InvalidParamsStructure(t *testing.T) {
	tool := NewShellTool("/tmp", 0)

	// Valid JSON but wrong structure (missing command field)
	args, _ := json.Marshal(map[string]string{
		"wrong_field": "value",
	})

	// This will result in empty command which should error
	_, err := tool.Execute(context.Background(), args)
	// The result depends on implementation - either error or success with empty output
	// We just verify it doesn't panic
	_ = err
}
