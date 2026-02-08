// Package approval provides permission approval management for tool execution.
package approval

import (
	"fmt"
	"sync"
)

// Level represents the approval level for tool execution.
type Level int

const (
	// LevelPerRequest asks for approval before each tool execution.
	LevelPerRequest Level = iota
	// LevelSession auto-approves tools for the current session.
	LevelSession
	// LevelYOLO auto-approves all tools (dangerous!).
	LevelYOLO
)

// String returns the string representation of the approval level.
func (l Level) String() string {
	switch l {
	case LevelPerRequest:
		return "per-request"
	case LevelSession:
		return "session"
	case LevelYOLO:
		return "yolo"
	default:
		return "unknown"
	}
}

// Manager handles tool execution approval decisions.
type Manager struct {
	mu sync.RWMutex

	// Current approval level
	level Level

	// Session-approved tools (tool name -> approved)
	sessionApproved map[string]bool

	// Tool-specific risk levels (tool name -> is dangerous)
	dangerousTools map[string]bool
}

// NewManager creates a new approval manager.
func NewManager(yolo bool) *Manager {
	level := LevelPerRequest
	if yolo {
		level = LevelYOLO
	}

	return &Manager{
		level:           level,
		sessionApproved: make(map[string]bool),
		dangerousTools: map[string]bool{
			"shell": true, // Shell commands are considered dangerous
			"file":  false, // File operations are generally safe
		},
	}
}

// GetLevel returns the current approval level.
func (m *Manager) GetLevel() Level {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.level
}

// SetLevel sets the approval level.
func (m *Manager) SetLevel(level Level) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.level = level
}

// IsApproved checks if a tool call is approved for execution.
// Returns true if approved, false if user approval is needed.
func (m *Manager) IsApproved(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// YOLO mode: always approve
	if m.level == LevelYOLO {
		return true
	}

	// Session mode: check if tool is session-approved
	if m.level == LevelSession {
		return m.sessionApproved[toolName]
	}

	// Per-request mode: need explicit approval each time
	return false
}

// ApproveOnce approves a tool for a single execution.
func (m *Manager) ApproveOnce(toolName string) {
	// No state change needed for single approval
}

// ApproveForSession approves a tool for the entire session.
func (m *Manager) ApproveForSession(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionApproved[toolName] = true
}

// RevokeSessionApproval removes session approval for a tool.
func (m *Manager) RevokeSessionApproval(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessionApproved, toolName)
}

// IsSessionApproved checks if a tool is approved for the session.
func (m *Manager) IsSessionApproved(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionApproved[toolName]
}

// IsDangerous checks if a tool is considered dangerous.
func (m *Manager) IsDangerous(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dangerousTools[toolName]
}

// SetDangerous marks a tool as dangerous or safe.
func (m *Manager) SetDangerous(toolName string, dangerous bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dangerousTools[toolName] = dangerous
}

// GetSessionApprovedTools returns a list of all session-approved tools.
func (m *Manager) GetSessionApprovedTools() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.sessionApproved))
	for tool := range m.sessionApproved {
		result = append(result, tool)
	}
	return result
}

// ClearSessionApprovals removes all session approvals.
func (m *Manager) ClearSessionApprovals() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionApproved = make(map[string]bool)
}

// ApprovalRequest represents a request for user approval.
type ApprovalRequest struct {
	ToolCallID string
	ToolName   string
	Arguments  string
	IsDangerous bool
}

// String returns a human-readable description of the approval request.
func (r *ApprovalRequest) String() string {
	dangerous := ""
	if r.IsDangerous {
		dangerous = " [DANGEROUS]"
	}
	return fmt.Sprintf("%s%s: %s", r.ToolName, dangerous, r.Arguments)
}

// NewApprovalRequest creates a new approval request.
func (m *Manager) NewApprovalRequest(callID, toolName, args string) *ApprovalRequest {
	return &ApprovalRequest{
		ToolCallID:  callID,
		ToolName:    toolName,
		Arguments:   args,
		IsDangerous: m.IsDangerous(toolName),
	}
}
