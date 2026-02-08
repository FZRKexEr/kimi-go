// Package session provides session management for kimi-go.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Session represents a user session.
type Session struct {
	ID          string    `json:"id"`
	WorkDir     string    `json:"work_dir"`
	ContextFile string    `json:"context_file"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SessionStore defines the interface for session storage.
type SessionStore interface {
	Save(session *Session) error
	Load(id string) (*Session, error)
	Delete(id string) error
	List() ([]*Session, error)
}

// FileSessionStore implements SessionStore using file system.
type FileSessionStore struct {
	baseDir string
}

// NewFileSessionStore creates a new file-based session store.
func NewFileSessionStore(baseDir string) (*FileSessionStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	return &FileSessionStore{baseDir: baseDir}, nil
}

// Save saves a session to file.
func (s *FileSessionStore) Save(session *Session) error {
	path := filepath.Join(s.baseDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	return nil
}

// Load loads a session from file.
func (s *FileSessionStore) Load(id string) (*Session, error) {
	path := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	return &session, nil
}

// Delete deletes a session.
func (s *FileSessionStore) Delete(id string) error {
	path := filepath.Join(s.baseDir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}
	return nil
}

// List lists all sessions.
func (s *FileSessionStore) List() ([]*Session, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		session, err := s.Load(id)
		if err != nil {
			continue // Skip invalid sessions
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// Create creates a new session.
func Create(workDir string) (*Session, error) {
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Get user's home directory for storing session data
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".kimi", "sessions")
	contextsDir := filepath.Join(homeDir, ".kimi", "contexts")

	for _, dir := range []string{sessionsDir, contextsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	id := uuid.New().String()
	session := &Session{
		ID:          id,
		WorkDir:     workDir,
		ContextFile: filepath.Join(contextsDir, id+".json"),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return session, nil
}

// Continue continues an existing session.
func Continue(id string) (*Session, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".kimi", "sessions")
	store, err := NewFileSessionStore(sessionsDir)
	if err != nil {
		return nil, err
	}

	return store.Load(id)
}

// Save saves the session.
func (s *Session) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".kimi", "sessions")
	store, err := NewFileSessionStore(sessionsDir)
	if err != nil {
		return err
	}

	s.UpdatedAt = time.Now()
	return store.Save(s)
}
