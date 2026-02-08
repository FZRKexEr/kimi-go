package approval

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name     string
		yolo     bool
		expected Level
	}{
		{"default mode", false, LevelPerRequest},
		{"yolo mode", true, LevelYOLO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.yolo)
			if m.GetLevel() != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, m.GetLevel())
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelPerRequest, "per-request"},
		{LevelSession, "session"},
		{LevelYOLO, "yolo"},
		{Level(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestIsApproved(t *testing.T) {
	tests := []struct {
		name       string
		level      Level
		toolName   string
		preApprove bool
		expected   bool
	}{
		{"yolo approves all", LevelYOLO, "shell", false, true},
		{"per-request denies", LevelPerRequest, "file", false, false},
		{"session approves approved tool", LevelSession, "file", true, true},
		{"session denies unapproved tool", LevelSession, "shell", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(false)
			m.SetLevel(tt.level)
			if tt.preApprove {
				m.ApproveForSession(tt.toolName)
			}
			if got := m.IsApproved(tt.toolName); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestApproveForSession(t *testing.T) {
	m := NewManager(false)
	m.SetLevel(LevelSession)

	if m.IsApproved("shell") {
		t.Error("shell should not be approved initially")
	}

	m.ApproveForSession("shell")
	if !m.IsApproved("shell") {
		t.Error("shell should be approved after ApproveForSession")
	}

	// Other tools should not be approved
	if m.IsApproved("file") {
		t.Error("file should not be approved")
	}
}

func TestRevokeSessionApproval(t *testing.T) {
	m := NewManager(false)
	m.SetLevel(LevelSession)

	m.ApproveForSession("shell")
	if !m.IsApproved("shell") {
		t.Error("shell should be approved")
	}

	m.RevokeSessionApproval("shell")
	if m.IsApproved("shell") {
		t.Error("shell should not be approved after revoke")
	}
}

func TestIsDangerous(t *testing.T) {
	m := NewManager(false)

	if !m.IsDangerous("shell") {
		t.Error("shell should be dangerous")
	}

	if m.IsDangerous("file") {
		t.Error("file should not be dangerous")
	}

	if m.IsDangerous("unknown") {
		t.Error("unknown tool should not be dangerous by default")
	}
}

func TestSetDangerous(t *testing.T) {
	m := NewManager(false)

	m.SetDangerous("file", true)
	if !m.IsDangerous("file") {
		t.Error("file should be dangerous after SetDangerous")
	}

	m.SetDangerous("shell", false)
	if m.IsDangerous("shell") {
		t.Error("shell should not be dangerous after SetDangerous")
	}
}

func TestGetSessionApprovedTools(t *testing.T) {
	m := NewManager(false)

	// Initially empty
	if len(m.GetSessionApprovedTools()) != 0 {
		t.Error("should have no approved tools initially")
	}

	m.ApproveForSession("shell")
	m.ApproveForSession("file")

	approved := m.GetSessionApprovedTools()
	if len(approved) != 2 {
		t.Errorf("expected 2 approved tools, got %d", len(approved))
	}
}

func TestClearSessionApprovals(t *testing.T) {
	m := NewManager(false)
	m.SetLevel(LevelSession)

	m.ApproveForSession("shell")
	m.ApproveForSession("file")

	m.ClearSessionApprovals()

	if m.IsApproved("shell") {
		t.Error("shell should not be approved after clear")
	}
	if m.IsApproved("file") {
		t.Error("file should not be approved after clear")
	}
	if len(m.GetSessionApprovedTools()) != 0 {
		t.Error("should have no approved tools after clear")
	}
}

func TestNewApprovalRequest(t *testing.T) {
	m := NewManager(false)

	req := m.NewApprovalRequest("call-123", "shell", "{\"command\":\"ls\"}")

	if req.ToolCallID != "call-123" {
		t.Errorf("expected call ID call-123, got %s", req.ToolCallID)
	}
	if req.ToolName != "shell" {
		t.Errorf("expected tool name shell, got %s", req.ToolName)
	}
	if !req.IsDangerous {
		t.Error("shell should be marked as dangerous")
	}
}

func TestApprovalRequestString(t *testing.T) {
	tests := []struct {
		name     string
		req      ApprovalRequest
		expected string
	}{
		{
			name:     "dangerous tool",
			req:      ApprovalRequest{ToolName: "shell", Arguments: "{\"cmd\":\"rm\"}", IsDangerous: true},
			expected: "shell [DANGEROUS]: {\"cmd\":\"rm\"}",
		},
		{
			name:     "safe tool",
			req:      ApprovalRequest{ToolName: "file", Arguments: "{\"path\":\"/tmp\"}", IsDangerous: false},
			expected: "file: {\"path\":\"/tmp\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
