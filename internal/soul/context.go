package soul

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"kimi-go/internal/wire"
)

// Context manages conversation history and state.
type Context struct {
	mu       sync.RWMutex
	messages []wire.Message
	filePath string
	modified bool
}

// NewContext creates a new context.
func NewContext(filePath string) *Context {
	return &Context{
		messages: make([]wire.Message, 0),
		filePath: filePath,
		modified: false,
	}
}

// AddMessage adds a message to the context.
func (c *Context) AddMessage(msg wire.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
	c.modified = true
}

// AddMessages adds multiple messages to the context.
func (c *Context) AddMessages(msgs ...wire.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msgs...)
	c.modified = true
}

// GetMessages returns all messages in the context.
func (c *Context) GetMessages() []wire.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]wire.Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// GetLastNMessages returns the last n messages.
func (c *Context) GetLastNMessages(n int) []wire.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if n >= len(c.messages) {
		result := make([]wire.Message, len(c.messages))
		copy(result, c.messages)
		return result
	}
	result := make([]wire.Message, n)
	copy(result, c.messages[len(c.messages)-n:])
	return result
}

// Clear clears all messages.
func (c *Context) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = make([]wire.Message, 0)
	c.modified = true
}

// Save saves the context to file.
func (c *Context) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.modified {
		return nil
	}

	data, err := json.MarshalIndent(c.messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := os.WriteFile(c.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	c.modified = false
	return nil
}

// Restore loads the context from file.
func (c *Context) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := os.Stat(c.filePath); os.IsNotExist(err) {
		return nil // No existing context
	}

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("failed to read context file: %w", err)
	}

	var messages []wire.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("failed to unmarshal context: %w", err)
	}

	c.messages = messages
	c.modified = false
	return nil
}

// Checkpoint creates a checkpoint of the current context.
func (c *Context) Checkpoint() wire.Checkpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, _ := json.Marshal(c.messages)
	return wire.Checkpoint{
		ID:        generateID(),
		Timestamp: time.Now(),
		Context:   data,
	}
}

// RestoreCheckpoint restores context from a checkpoint.
func (c *Context) RestoreCheckpoint(cp wire.Checkpoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var messages []wire.Message
	if err := json.Unmarshal(cp.Context, &messages); err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	c.messages = messages
	c.modified = true
	return nil
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
