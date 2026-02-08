package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTool_NameDescriptionParameters(t *testing.T) {
	ft := NewFileTool("/tmp")
	if ft.Name() != "file" {
		t.Errorf("expected name 'file', got %q", ft.Name())
	}
	if ft.Description() == "" {
		t.Error("description should not be empty")
	}
	var schema map[string]any
	if err := json.Unmarshal(ft.Parameters(), &schema); err != nil {
		t.Fatalf("Parameters() is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Error("schema type should be 'object'")
	}
}

func TestFileTool_ReadFile_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("line1\nline2\nline3\n"), 0644)

	ft := NewFileTool(dir)
	result, err := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "hello.txt"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("read failed: %s", r.Error)
	}
	if !strings.Contains(r.Content, "line1") {
		t.Errorf("expected content to contain 'line1', got %q", r.Content)
	}
}

func TestFileTool_ReadFile_NonExistent(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	result, err := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "does_not_exist.txt"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.(FileResult)
	if r.Success {
		t.Error("reading non-existent file should not succeed")
	}
	if r.Error == "" {
		t.Error("error message should not be empty")
	}
}

func TestFileTool_ReadFile_WithOffset(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lines.txt"), []byte("a\nb\nc\nd\ne\n"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "lines.txt",
		"offset": 2
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("read failed: %s", r.Error)
	}
	// offset=2 means skip first 2 lines, should start from "c"
	if !strings.HasPrefix(r.Content, "c") {
		t.Errorf("expected content to start with 'c', got %q", r.Content)
	}
}

func TestFileTool_ReadFile_WithLimit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lines.txt"), []byte("a\nb\nc\nd\ne\n"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "lines.txt",
		"offset": 1,
		"limit": 2
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("read failed: %s", r.Error)
	}
	lines := strings.Split(strings.TrimRight(r.Content, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}

func TestFileTool_ReadFile_OffsetBeyondEnd(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "short.txt"), []byte("ab\n"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "short.txt",
		"offset": 100
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("read failed: %s", r.Error)
	}
	if r.Content != "" {
		t.Errorf("expected empty content for large offset, got %q", r.Content)
	}
}

func TestFileTool_WriteFile_Success(t *testing.T) {
	dir := t.TempDir()
	ft := NewFileTool(dir)

	result, err := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "write",
		"path": "out.txt",
		"content": "hello world"
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("write failed: %s", r.Error)
	}

	// Verify file contents
	data, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestFileTool_WriteFile_CreateParentDirs(t *testing.T) {
	dir := t.TempDir()
	ft := NewFileTool(dir)

	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "write",
		"path": "sub/dir/deep.txt",
		"content": "nested"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("write failed: %s", r.Error)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "sub/dir/deep.txt"))
	if string(data) != "nested" {
		t.Errorf("expected 'nested', got %q", string(data))
	}
}

func TestFileTool_WriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	ft := NewFileTool(dir)
	path := filepath.Join(dir, "overwrite.txt")
	os.WriteFile(path, []byte("old"), 0644)

	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "write",
		"path": "overwrite.txt",
		"content": "new"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("write failed: %s", r.Error)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Errorf("expected 'new', got %q", string(data))
	}
}

func TestFileTool_ListDir_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "list",
		"path": "."
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("list failed: %s", r.Error)
	}
	if len(r.Files) != 3 {
		t.Errorf("expected 3 entries, got %d", len(r.Files))
	}

	// Check that subdir is marked as directory
	found := false
	for _, f := range r.Files {
		if f.Name == "subdir" && f.IsDir {
			found = true
		}
	}
	if !found {
		t.Error("subdir should be listed as a directory")
	}
}

func TestFileTool_ListDir_Empty(t *testing.T) {
	dir := t.TempDir()
	ft := NewFileTool(dir)

	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "list",
		"path": "."
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("list failed: %s", r.Error)
	}
	if len(r.Files) != 0 {
		t.Errorf("expected 0 entries for empty dir, got %d", len(r.Files))
	}
}

func TestFileTool_ListDir_NonExistent(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "list",
		"path": "no_such_dir"
	}`))
	r := result.(FileResult)
	if r.Success {
		t.Error("listing non-existent directory should fail")
	}
}

func TestFileTool_DeleteFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delete_me.txt")
	os.WriteFile(path, []byte("bye"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "delete",
		"path": "delete_me.txt"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("delete failed: %s", r.Error)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestFileTool_DeleteFile_NonExistent(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "delete",
		"path": "nope.txt"
	}`))
	r := result.(FileResult)
	// os.RemoveAll returns nil for non-existent paths
	if !r.Success {
		t.Error("deleting non-existent file should succeed (no-op)")
	}
}

func TestFileTool_DeleteDir_Recursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "parent", "child")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("deep"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "delete",
		"path": "parent"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("recursive delete failed: %s", r.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "parent")); !os.IsNotExist(err) {
		t.Error("directory should be recursively deleted")
	}
}

func TestFileTool_FileExists_True(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("x"), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "exists",
		"path": "exists.txt"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Error("existing file should return Success=true")
	}
}

func TestFileTool_FileExists_False(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "exists",
		"path": "nope.txt"
	}`))
	r := result.(FileResult)
	if r.Success {
		t.Error("non-existent file should return Success=false")
	}
}

func TestFileTool_ResolvePath_Absolute(t *testing.T) {
	ft := NewFileTool("/home/user")
	path := ft.resolvePath("/etc/passwd")
	if path != "/etc/passwd" {
		t.Errorf("absolute path should not be modified, got %q", path)
	}
}

func TestFileTool_ResolvePath_Relative(t *testing.T) {
	ft := NewFileTool("/home/user")
	path := ft.resolvePath("file.txt")
	expected := filepath.Join("/home/user", "file.txt")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestFileTool_ResolvePath_NoWorkDir(t *testing.T) {
	ft := NewFileTool("")
	path := ft.resolvePath("file.txt")
	if path != "file.txt" {
		t.Errorf("expected 'file.txt' when no workDir, got %q", path)
	}
}

func TestFileTool_InvalidOperation(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	_, err := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "compress",
		"path": "test.txt"
	}`))
	if err == nil {
		t.Error("invalid operation should return error")
	}
}

func TestFileTool_InvalidJSON(t *testing.T) {
	ft := NewFileTool(t.TempDir())
	_, err := ft.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("invalid JSON should return error")
	}
}

func TestFileTool_SetWorkDir(t *testing.T) {
	ft := NewFileTool("/old")
	ft.SetWorkDir("/new")

	path := ft.resolvePath("test.txt")
	expected := filepath.Join("/new", "test.txt")
	if path != expected {
		t.Errorf("expected %q after SetWorkDir, got %q", expected, path)
	}
}

func TestFileTool_ReadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte(""), 0644)

	ft := NewFileTool(dir)
	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "read",
		"path": "empty.txt"
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("reading empty file should succeed: %s", r.Error)
	}
	if r.Content != "" {
		t.Errorf("expected empty content, got %q", r.Content)
	}
}

func TestFileTool_WriteFile_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	ft := NewFileTool(dir)

	result, _ := ft.Execute(context.Background(), json.RawMessage(`{
		"operation": "write",
		"path": "empty.txt",
		"content": ""
	}`))
	r := result.(FileResult)
	if !r.Success {
		t.Fatalf("writing empty content should succeed: %s", r.Error)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}
