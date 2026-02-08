// Package session provides session management for kimi-go.
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileSessionStore(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
}

func TestNewFileSessionStore_CreateDir(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "new", "subdir")

	store, err := NewFileSessionStore(nonExistentDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	// Verify directory was created
	if _, err := os.Stat(nonExistentDir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}

func TestFileSessionStore_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	session := &Session{
		ID:          "test-session-123",
		WorkDir:     "/tmp/test",
		ContextFile: "/tmp/test/context.json",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save
	if err := store.Save(session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Load
	loaded, err := store.Load("test-session-123")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, loaded.ID)
	}

	if loaded.WorkDir != session.WorkDir {
		t.Errorf("Expected WorkDir %s, got %s", session.WorkDir, loaded.WorkDir)
	}
}

func TestFileSessionStore_Load_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = store.Load("non-existent-session")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestFileSessionStore_Delete(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	session := &Session{
		ID:        "test-delete-123",
		WorkDir:   "/tmp/test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save
	if err := store.Save(session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Delete
	if err := store.Delete("test-delete-123"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify deleted
	_, err = store.Load("test-delete-123")
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestFileSessionStore_Delete_NotExist(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not error for non-existent session
	if err := store.Delete("non-existent"); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFileSessionStore_List(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileSessionStore(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Create multiple sessions
	sessions := []*Session{
		{
			ID:        "session-1",
			WorkDir:   "/tmp/1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "session-2",
			WorkDir:   "/tmp/2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, s := range sessions {
		if err := store.Save(s); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}
	}

	// List
	list, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(list))
	}
}

func TestCreate(t *testing.T) {
	tempDir := t.TempDir()

	// Set home directory to temp dir for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	session, err := Create(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if session.ID == "" {
		t.Error("Expected non-empty session ID")
	}

	if session.WorkDir != tempDir {
		t.Errorf("Expected WorkDir %s, got %s", tempDir, session.WorkDir)
	}

	if session.ContextFile == "" {
		t.Error("Expected non-empty ContextFile")
	}

	if session.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}

	if session.UpdatedAt.IsZero() {
		t.Error("Expected non-zero UpdatedAt")
	}
}

func TestCreate_DefaultWorkDir(t *testing.T) {
	// Set home directory to temp dir for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create with empty workDir should use current directory
	session, err := Create("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	// WorkDir should be current directory
	if session.WorkDir == "" {
		t.Error("Expected non-empty WorkDir")
	}
}

func TestSessionSave(t *testing.T) {
	tempDir := t.TempDir()

	// Set home directory to temp dir for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	session, err := Create(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Save
	if err := session.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify session file exists
	sessionsDir := tempDir + "/.kimi/sessions"
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("Expected sessions directory to exist")
	}
}

func TestContinue(t *testing.T) {
	tempDir := t.TempDir()

	// Set home directory to temp dir for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create a session first
	session, err := Create(tempDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Save it
	if err := session.Save(); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Continue the session
	loaded, err := Continue(session.ID)
	if err != nil {
		t.Fatalf("Failed to continue session: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, loaded.ID)
	}

	if loaded.WorkDir != session.WorkDir {
		t.Errorf("Expected WorkDir %s, got %s", session.WorkDir, loaded.WorkDir)
	}
}

func TestContinue_NotFound(t *testing.T) {
	tempDir := t.TempDir()

	// Set home directory to temp dir for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Try to continue non-existent session
	_, err := Continue("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}
