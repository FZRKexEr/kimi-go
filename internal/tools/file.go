package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileTool provides file operations.
type FileTool struct {
	workDir string
}

// FileReadParams represents parameters for file read operation.
type FileReadParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// FileWriteParams represents parameters for file write operation.
type FileWriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
}

// FileListParams represents parameters for file list operation.
type FileListParams struct {
	Path    string `json:"path,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// FileResult represents the result of a file operation.
type FileResult struct {
	Success bool       `json:"success"`
	Content string     `json:"content,omitempty"`
	Error   string     `json:"error,omitempty"`
	Files   []FileInfo `json:"files,omitempty"`
}

// FileInfo represents file information.
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}

// NewFileTool creates a new file tool.
func NewFileTool(workDir string) *FileTool {
	return &FileTool{workDir: workDir}
}

// Name returns the tool name.
func (t *FileTool) Name() string {
	return "file"
}

// Description returns the tool description.
func (t *FileTool) Description() string {
	return "File operations including read, write, list, and search."
}

// Parameters returns the JSON schema for tool parameters.
func (t *FileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"operation": {
				"type": "string",
				"enum": ["read", "write", "list", "delete", "exists"],
				"description": "The file operation to perform"
			},
			"path": {
				"type": "string",
				"description": "The file or directory path"
			},
			"content": {
				"type": "string",
				"description": "Content to write (for write operation)"
			},
			"offset": {
				"type": "integer",
				"description": "Offset to start reading from"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of lines to read"
			}
		},
		"required": ["operation", "path"]
	}`)
}

// Execute executes the file tool.
func (t *FileTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Content   string `json:"content,omitempty"`
		Offset    int    `json:"offset,omitempty"`
		Limit     int    `json:"limit,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Resolve path relative to work directory
	path := t.resolvePath(params.Path)

	switch params.Operation {
	case "read":
		return t.readFile(path, params.Offset, params.Limit)
	case "write":
		return t.writeFile(path, params.Content)
	case "list":
		return t.listDir(path)
	case "delete":
		return t.deleteFile(path)
	case "exists":
		return t.fileExists(path)
	default:
		return nil, fmt.Errorf("unknown operation: %s", params.Operation)
	}
}

func (t *FileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if t.workDir != "" {
		return filepath.Join(t.workDir, path)
	}
	return path
}

func (t *FileTool) readFile(path string, offset, limit int) (FileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	content := string(data)

	// Handle offset and limit for line-based reading
	if offset > 0 || limit > 0 {
		lines := strings.Split(content, "\n")
		if offset > len(lines) {
			offset = len(lines)
		}
		end := len(lines)
		if limit > 0 && offset+limit < end {
			end = offset + limit
		}
		content = strings.Join(lines[offset:end], "\n")
	}

	return FileResult{
		Success: true,
		Content: content,
	}, nil
}

func (t *FileTool) writeFile(path string, content string) (FileResult, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return FileResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return FileResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return FileResult{
		Success: true,
	}, nil
}

func (t *FileTool) listDir(path string) (FileResult, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return FileResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Unix(),
		})
	}

	return FileResult{
		Success: true,
		Files:   files,
	}, nil
}

func (t *FileTool) deleteFile(path string) (FileResult, error) {
	if err := os.RemoveAll(path); err != nil {
		return FileResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return FileResult{Success: true}, nil
}

func (t *FileTool) fileExists(path string) (FileResult, error) {
	_, err := os.Stat(path)
	exists := err == nil
	return FileResult{
		Success: exists,
	}, nil
}

// SetWorkDir sets the working directory.
func (t *FileTool) SetWorkDir(dir string) {
	t.workDir = dir
}
