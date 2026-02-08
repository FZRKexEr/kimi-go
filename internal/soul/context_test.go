package soul

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kimi-go/internal/wire"
)

func testMsg(msgType wire.MessageType, text string) wire.Message {
	return wire.Message{
		Type: msgType,
		Content: []wire.ContentPart{{
			Type: "text",
			Text: text,
		}},
		Timestamp: time.Now(),
	}
}

func TestNewContext(t *testing.T) {
	ctx := NewContext("/tmp/test_ctx.json")
	if ctx == nil {
		t.Fatal("NewContext returned nil")
	}
	if len(ctx.GetMessages()) != 0 {
		t.Error("new context should have 0 messages")
	}
}

func TestContext_AddMessage(t *testing.T) {
	ctx := NewContext("")
	msg := testMsg(wire.MessageTypeUserInput, "hello")
	ctx.AddMessage(msg)

	msgs := ctx.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", msgs[0].Content[0].Text)
	}
}

func TestContext_AddMessages(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessages(
		testMsg(wire.MessageTypeUserInput, "a"),
		testMsg(wire.MessageTypeAssistant, "b"),
		testMsg(wire.MessageTypeUserInput, "c"),
	)
	if len(ctx.GetMessages()) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(ctx.GetMessages()))
	}
}

func TestContext_GetMessages_ReturnsCopy(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessage(testMsg(wire.MessageTypeUserInput, "original"))

	msgs := ctx.GetMessages()
	// Append to returned slice should not affect internal state
	msgs = append(msgs, testMsg(wire.MessageTypeUserInput, "extra"))

	original := ctx.GetMessages()
	if len(original) != 1 {
		t.Error("GetMessages should return a copy — append should not affect internal state")
	}
}

func TestContext_GetLastNMessages_LessThanTotal(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessages(
		testMsg(wire.MessageTypeUserInput, "a"),
		testMsg(wire.MessageTypeUserInput, "b"),
		testMsg(wire.MessageTypeUserInput, "c"),
	)

	msgs := ctx.GetLastNMessages(2)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content[0].Text != "b" || msgs[1].Content[0].Text != "c" {
		t.Errorf("expected [b, c], got [%s, %s]",
			msgs[0].Content[0].Text, msgs[1].Content[0].Text)
	}
}

func TestContext_GetLastNMessages_MoreThanTotal(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessage(testMsg(wire.MessageTypeUserInput, "only"))

	msgs := ctx.GetLastNMessages(10)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestContext_GetLastNMessages_ReturnsCopy(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessages(
		testMsg(wire.MessageTypeUserInput, "a"),
		testMsg(wire.MessageTypeUserInput, "b"),
	)

	msgs := ctx.GetLastNMessages(1)
	msgs = append(msgs, testMsg(wire.MessageTypeUserInput, "extra"))

	original := ctx.GetLastNMessages(1)
	if len(original) != 1 {
		t.Error("GetLastNMessages should return a copy — append should not affect internal state")
	}
}

func TestContext_GetLastNMessages_Zero(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessage(testMsg(wire.MessageTypeUserInput, "a"))

	msgs := ctx.GetLastNMessages(0)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for n=0, got %d", len(msgs))
	}
}

func TestContext_Clear(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessages(
		testMsg(wire.MessageTypeUserInput, "a"),
		testMsg(wire.MessageTypeUserInput, "b"),
	)
	ctx.Clear()
	if len(ctx.GetMessages()) != 0 {
		t.Error("Clear should remove all messages")
	}
}

func TestContext_SaveRestore(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "ctx.json")

	// Save
	ctx1 := NewContext(filePath)
	ctx1.AddMessages(
		testMsg(wire.MessageTypeUserInput, "hello"),
		testMsg(wire.MessageTypeAssistant, "hi there"),
	)
	if err := ctx1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("context file not created: %v", err)
	}

	// Restore into new context
	ctx2 := NewContext(filePath)
	if err := ctx2.Restore(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	msgs := ctx2.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after restore, got %d", len(msgs))
	}
	if msgs[0].Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", msgs[0].Content[0].Text)
	}
	if msgs[1].Content[0].Text != "hi there" {
		t.Errorf("expected 'hi there', got %q", msgs[1].Content[0].Text)
	}
}

func TestContext_Save_NotModified(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "ctx.json")

	ctx := NewContext(filePath)
	// No modifications — save should be a no-op
	if err := ctx.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	// File should not exist since nothing was saved
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Save should not create file when not modified")
	}
}

func TestContext_Restore_NonExistent(t *testing.T) {
	ctx := NewContext("/tmp/does_not_exist_12345.json")
	// Should not error — just returns empty
	if err := ctx.Restore(); err != nil {
		t.Fatalf("Restore of non-existent file should not error: %v", err)
	}
	if len(ctx.GetMessages()) != 0 {
		t.Error("messages should be empty after restoring non-existent file")
	}
}

func TestContext_Restore_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.json")
	os.WriteFile(filePath, []byte("not valid json{{{"), 0644)

	ctx := NewContext(filePath)
	err := ctx.Restore()
	if err == nil {
		t.Error("Restore should fail on corrupted JSON")
	}
}

func TestContext_Checkpoint_RestoreCheckpoint(t *testing.T) {
	ctx := NewContext("")
	ctx.AddMessages(
		testMsg(wire.MessageTypeUserInput, "before"),
		testMsg(wire.MessageTypeAssistant, "response"),
	)

	cp := ctx.Checkpoint()
	if cp.ID == "" {
		t.Error("checkpoint ID should not be empty")
	}

	// Clear and verify empty
	ctx.Clear()
	if len(ctx.GetMessages()) != 0 {
		t.Error("should be empty after clear")
	}

	// Restore checkpoint
	if err := ctx.RestoreCheckpoint(cp); err != nil {
		t.Fatalf("RestoreCheckpoint failed: %v", err)
	}

	msgs := ctx.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after checkpoint restore, got %d", len(msgs))
	}
	if msgs[0].Content[0].Text != "before" {
		t.Errorf("expected 'before', got %q", msgs[0].Content[0].Text)
	}
}

func TestContext_RestoreCheckpoint_InvalidJSON(t *testing.T) {
	ctx := NewContext("")
	cp := wire.Checkpoint{
		ID:      "test",
		Context: []byte("invalid json"),
	}
	err := ctx.RestoreCheckpoint(cp)
	if err == nil {
		t.Error("RestoreCheckpoint should fail on invalid JSON")
	}
}

func TestContext_ConcurrentAccess(t *testing.T) {
	ctx := NewContext("")

	var wg sync.WaitGroup
	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx.AddMessage(testMsg(wire.MessageTypeUserInput, "msg"))
		}(i)
	}
	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ctx.GetMessages()
			_ = ctx.GetLastNMessages(5)
		}()
	}
	wg.Wait()

	msgs := ctx.GetMessages()
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages after concurrent adds, got %d", len(msgs))
	}
}
