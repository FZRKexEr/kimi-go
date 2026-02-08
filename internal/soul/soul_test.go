package soul

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"kimi-go/internal/llm"
	"kimi-go/internal/tools"
	"kimi-go/internal/wire"
)

// --- helpers ---

// mockLLMServer creates a test HTTP server that returns a sequence of ChatResponses.
func mockLLMServer(t *testing.T, responses []llm.ChatResponse) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	callIndex := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		idx := callIndex
		callIndex++
		mu.Unlock()

		if idx >= len(responses) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"no more responses"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses[idx])
	}))
}

func textResponse(content string) llm.ChatResponse {
	return llm.ChatResponse{
		ID:    "resp-1",
		Model: "test",
		Choices: []struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			Delta        llm.Message `json:"delta"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index:        0,
				Message:      llm.Message{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
	}
}

func toolCallResponse(callID, toolName, args string) llm.ChatResponse {
	return llm.ChatResponse{
		ID:    "resp-tc",
		Model: "test",
		Choices: []struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			Delta        llm.Message `json:"delta"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCallInfo{
						{
							ID:   callID,
							Type: "function",
							Function: llm.FunctionCall{
								Name:      toolName,
								Arguments: args,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}
}

func setupSoul(t *testing.T, server *httptest.Server) *Soul {
	t.Helper()
	runtime := NewRuntime(t.TempDir(), true)
	runtime.LLMClient = llm.NewClient(llm.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10 * time.Second,
	})
	runtime.MaxSteps = 10

	shellTool := tools.NewShellTool(runtime.WorkDir, 5*time.Second)
	runtime.RegisterTool(shellTool)

	agent := NewAgent("test", "You are a test assistant.", runtime)
	agent.AddTool("shell")

	ctx := NewContext("")
	return NewSoul(agent, ctx)
}

// --- Runtime tests ---

func TestNewRuntime(t *testing.T) {
	rt := NewRuntime("/tmp/work", false)
	if rt.WorkDir != "/tmp/work" {
		t.Errorf("expected /tmp/work, got %q", rt.WorkDir)
	}
	if rt.YOLO {
		t.Error("YOLO should be false")
	}
	if rt.MaxSteps != 100 {
		t.Errorf("expected MaxSteps=100, got %d", rt.MaxSteps)
	}
	if rt.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", rt.MaxRetries)
	}
	if rt.Tools == nil {
		t.Error("Tools should not be nil")
	}
	if rt.LLMClient != nil {
		t.Error("LLMClient should be nil by default")
	}
}

func TestRuntime_RegisterTool(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	tool := tools.NewShellTool(rt.WorkDir, 0)

	if err := rt.RegisterTool(tool); err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}
	// Duplicate should fail
	if err := rt.RegisterTool(tool); err == nil {
		t.Error("duplicate registration should fail")
	}
}

// --- Agent tests ---

func TestNewAgent(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("myagent", "system prompt", rt)

	if agent.Name != "myagent" {
		t.Errorf("expected 'myagent', got %q", agent.Name)
	}
	if agent.SystemPrompt != "system prompt" {
		t.Errorf("unexpected system prompt: %q", agent.SystemPrompt)
	}
	if len(agent.Tools) != 0 {
		t.Error("tools should be empty initially")
	}
}

func TestAgent_AddTool(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	agent.AddTool("shell")
	agent.AddTool("file")

	if len(agent.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(agent.Tools))
	}
}

// --- Soul construction tests ---

func TestNewSoul(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	if s.Agent != agent {
		t.Error("Agent should match")
	}
	if s.Context != ctx {
		t.Error("Context should match")
	}
	if s.IsRunning() {
		t.Error("should not be running initially")
	}
}

// --- Soul Run / Cancel tests ---

func TestSoul_RunAndCancel(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	soulCtx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(soulCtx)
	}()

	// Wait until running
	time.Sleep(50 * time.Millisecond)
	if !s.IsRunning() {
		t.Error("should be running after start")
	}

	cancel()

	err := <-errCh
	if err == nil {
		t.Error("expected error after cancel")
	}

	if s.IsRunning() {
		t.Error("should not be running after cancel")
	}
}

func TestSoul_AlreadyRunning(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	soulCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Run(soulCtx)
	time.Sleep(50 * time.Millisecond)

	err := s.Run(soulCtx)
	if err == nil {
		t.Error("second Run should fail")
	}
}

func TestSoul_Cancel_Concurrent(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Call Cancel concurrently — should not panic
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Cancel()
		}()
	}
	wg.Wait()
}

// --- SendMessage tests ---

func TestSoul_SendMessage(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	msg := *wire.NewTextMessage(wire.MessageTypeUserInput, "hello")
	if err := s.SendMessage(msg); err != nil {
		t.Fatalf("SendMessage should succeed: %v", err)
	}
}

// --- processMessage tests ---

func TestSoul_ProcessMessage_UnknownType(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	err := s.processMessage(context.Background(), wire.Message{
		Type: "unknown_type",
	})
	if err == nil {
		t.Error("unknown message type should return error")
	}
}

// --- processWithLLM tests ---

func TestSoul_ProcessWithLLM_NoClient(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	// No LLMClient set
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	var received []wire.Message
	s.OnMessage = func(msg wire.Message) {
		received = append(received, msg)
	}

	userMsg := testMsg(wire.MessageTypeUserInput, "hello")
	err := s.processWithLLM(context.Background(), userMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0].Type != wire.MessageTypeAssistant {
		t.Error("expected assistant message")
	}
	// Should contain warning about LLM not configured
	text := received[0].Content[0].Text
	if text == "" {
		t.Error("should return a fallback message")
	}
}

func TestSoul_ProcessWithLLM_TextResponse(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		textResponse("Hello from LLM!"),
	})
	defer server.Close()

	s := setupSoul(t, server)

	var received []wire.Message
	s.OnMessage = func(msg wire.Message) {
		received = append(received, msg)
	}

	userMsg := testMsg(wire.MessageTypeUserInput, "hello")
	err := s.processWithLLM(context.Background(), userMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0].Content[0].Text != "Hello from LLM!" {
		t.Errorf("expected 'Hello from LLM!', got %q", received[0].Content[0].Text)
	}
}

func TestSoul_ProcessWithLLM_ToolCall(t *testing.T) {
	// First response: tool call, Second response: text summary
	server := mockLLMServer(t, []llm.ChatResponse{
		toolCallResponse("call_1", "shell", `{"command":"echo hello"}`),
		textResponse("The command output: hello"),
	})
	defer server.Close()

	s := setupSoul(t, server)

	var messages []wire.Message
	var toolCalls []tools.ToolCall
	var toolResults []tools.ToolResult

	s.OnMessage = func(msg wire.Message) {
		messages = append(messages, msg)
	}
	s.OnToolCall = func(tc tools.ToolCall) {
		toolCalls = append(toolCalls, tc)
	}
	s.OnToolResult = func(tr tools.ToolResult) {
		toolResults = append(toolResults, tr)
	}

	userMsg := testMsg(wire.MessageTypeUserInput, "run echo hello")
	err := s.processWithLLM(context.Background(), userMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: tool_call msg, tool_result msg, assistant msg
	if len(toolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(toolCalls))
	}
	if len(toolResults) != 1 {
		t.Errorf("expected 1 tool result, got %d", len(toolResults))
	}
	if toolCalls[0].Name != "shell" {
		t.Errorf("expected tool 'shell', got %q", toolCalls[0].Name)
	}
	if !toolResults[0].Success {
		t.Errorf("tool should succeed: %s", toolResults[0].Error)
	}

	// Last message should be the assistant text response
	lastMsg := messages[len(messages)-1]
	if lastMsg.Type != wire.MessageTypeAssistant {
		t.Errorf("last message should be assistant, got %s", lastMsg.Type)
	}
	if lastMsg.Content[0].Text != "The command output: hello" {
		t.Errorf("unexpected final response: %q", lastMsg.Content[0].Text)
	}
}

func TestSoul_ProcessWithLLM_ToolNotFound(t *testing.T) {
	// Call a non-existent tool
	server := mockLLMServer(t, []llm.ChatResponse{
		toolCallResponse("call_1", "nonexistent_tool", `{}`),
		textResponse("Tool not found, sorry."),
	})
	defer server.Close()

	s := setupSoul(t, server)

	var toolResults []tools.ToolResult
	s.OnMessage = func(msg wire.Message) {}
	s.OnToolCall = func(tc tools.ToolCall) {}
	s.OnToolResult = func(tr tools.ToolResult) {
		toolResults = append(toolResults, tr)
	}

	err := s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(toolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(toolResults))
	}
	if toolResults[0].Success {
		t.Error("tool result should indicate failure for missing tool")
	}
}

func TestSoul_ProcessWithLLM_MaxSteps(t *testing.T) {
	// Always return tool calls — should hit MaxSteps
	responses := make([]llm.ChatResponse, 15)
	for i := range responses {
		responses[i] = toolCallResponse(
			fmt.Sprintf("call_%d", i), "shell", `{"command":"echo loop"}`,
		)
	}

	server := mockLLMServer(t, responses)
	defer server.Close()

	s := setupSoul(t, server)
	s.runtime.MaxSteps = 3

	s.OnMessage = func(msg wire.Message) {}
	s.OnToolCall = func(tc tools.ToolCall) {}
	s.OnToolResult = func(tr tools.ToolResult) {}

	err := s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "loop"))
	if err == nil {
		t.Fatal("expected max steps error")
	}
	if err.Error() != "agent loop exceeded maximum steps (3)" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSoul_ProcessWithLLM_LLMError(t *testing.T) {
	// Server returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	}))
	defer server.Close()

	s := setupSoul(t, server)
	s.OnMessage = func(msg wire.Message) {}

	err := s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "test"))
	if err == nil {
		t.Fatal("expected error when LLM returns 500")
	}
}

func TestSoul_ProcessWithLLM_EmptyChoices(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		{ID: "resp", Choices: nil},
	})
	defer server.Close()

	s := setupSoul(t, server)
	s.OnMessage = func(msg wire.Message) {}

	err := s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "test"))
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

// --- buildLLMMessages tests ---

func TestSoul_BuildLLMMessages_WithSystemPrompt(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "You are helpful.", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	s.llmHistory = append(s.llmHistory, llm.Message{Role: "user", Content: "hi"})

	msgs := s.buildLLMMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message should be system, got %q", msgs[0].Role)
	}
	if msgs[0].Content != "You are helpful." {
		t.Errorf("unexpected system prompt: %q", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second message should be user, got %q", msgs[1].Role)
	}
}

func TestSoul_BuildLLMMessages_NoSystemPrompt(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt) // empty system prompt
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	s.llmHistory = append(s.llmHistory, llm.Message{Role: "user", Content: "hi"})

	msgs := s.buildLLMMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (no system), got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("first message should be user, got %q", msgs[0].Role)
	}
}

// --- buildToolDefs tests ---

func TestSoul_BuildToolDefs(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	rt.RegisterTool(tools.NewShellTool(rt.WorkDir, 0))
	rt.RegisterTool(tools.NewFileTool(rt.WorkDir))

	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	defs := s.buildToolDefs()
	if len(defs) != 2 {
		t.Fatalf("expected 2 tool defs, got %d", len(defs))
	}
	for _, d := range defs {
		if d.Type != "function" {
			t.Errorf("tool type should be 'function', got %q", d.Type)
		}
		if d.Function.Name == "" {
			t.Error("function name should not be empty")
		}
		if d.Function.Description == "" {
			t.Error("function description should not be empty")
		}
		if d.Function.Parameters == nil {
			t.Error("function parameters should not be nil")
		}
	}
}

func TestSoul_BuildToolDefs_Empty(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	defs := s.buildToolDefs()
	if defs != nil {
		t.Errorf("expected nil for empty tool set, got %v", defs)
	}
}

// --- executeToolCall tests ---

func TestSoul_ExecuteToolCall_Success(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	rt.RegisterTool(tools.NewShellTool(rt.WorkDir, 5*time.Second))

	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	call := tools.ToolCall{
		ID:        "call_1",
		Name:      "shell",
		Arguments: json.RawMessage(`{"command":"echo test_output"}`),
	}

	result, err := s.executeToolCall(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("tool execution should succeed: %s", result.Error)
	}
	if result.CallID != "call_1" {
		t.Errorf("expected call_id 'call_1', got %q", result.CallID)
	}
}

func TestSoul_ExecuteToolCall_ToolNotFound(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	call := tools.ToolCall{
		ID:        "call_1",
		Name:      "nonexistent",
		Arguments: json.RawMessage(`{}`),
	}

	result, err := s.executeToolCall(context.Background(), call)
	if err != nil {
		t.Fatalf("should not return go error: %v", err)
	}
	if result.Success {
		t.Error("should fail for missing tool")
	}
	if result.Error == "" {
		t.Error("error message should not be empty")
	}
}

// --- extractText tests ---

func TestExtractText(t *testing.T) {
	msg := wire.Message{
		Content: []wire.ContentPart{
			{Type: "text", Text: "hello world"},
		},
	}
	if got := extractText(msg); got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestExtractText_Empty(t *testing.T) {
	msg := wire.Message{}
	if got := extractText(msg); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractText_NonTextParts(t *testing.T) {
	msg := wire.Message{
		Content: []wire.ContentPart{
			{Type: "image", Data: []byte("binary")},
			{Type: "text", Text: "found it"},
		},
	}
	if got := extractText(msg); got != "found it" {
		t.Errorf("expected 'found it', got %q", got)
	}
}

// --- handleError tests ---

func TestSoul_HandleError(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	var received error
	s.OnError = func(err error) {
		received = err
	}

	testErr := fmt.Errorf("test error")
	s.handleError(testErr)

	if received != testErr {
		t.Errorf("expected test error, got %v", received)
	}
}

func TestSoul_HandleError_NoHandler(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Should not panic when OnError is nil
	s.handleError(fmt.Errorf("test"))
}

// --- Full integration: Run + SendMessage ---

func TestSoul_FullLoop_TextResponse(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		textResponse("Integration test response"),
	})
	defer server.Close()

	s := setupSoul(t, server)

	var received []wire.Message
	s.OnMessage = func(msg wire.Message) {
		received = append(received, msg)
	}

	soulCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Run(soulCtx)
	time.Sleep(50 * time.Millisecond)

	msg := *wire.NewTextMessage(wire.MessageTypeUserInput, "hello")
	if err := s.SendMessage(msg); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Wait for processing
	select {
	case <-s.DoneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for response")
	}

	// Should have received the assistant response
	found := false
	for _, m := range received {
		if m.Type == wire.MessageTypeAssistant {
			if m.Content[0].Text == "Integration test response" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to receive 'Integration test response'")
	}
}

func TestSoul_LLMHistory_Accumulates(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		textResponse("Response 1"),
		textResponse("Response 2"),
	})
	defer server.Close()

	s := setupSoul(t, server)
	s.OnMessage = func(msg wire.Message) {}

	// First message
	err := s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "msg1"))
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second message
	err = s.processWithLLM(context.Background(), testMsg(wire.MessageTypeUserInput, "msg2"))
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// LLM history should have: user1, assistant1, user2, assistant2
	if len(s.llmHistory) != 4 {
		t.Errorf("expected 4 history entries, got %d", len(s.llmHistory))
	}
}

// --- Token tracking and compression tests ---

func TestSoul_UpdateTokenCount(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	resp := &llm.ChatResponse{}
	resp.Usage.TotalTokens = 1500

	s.updateTokenCount(resp)

	if s.tokenCount != 1500 {
		t.Errorf("expected token count 1500, got %d", s.tokenCount)
	}
}

func TestSoul_ShouldCompress(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Set small context size for testing
	s.SetMaxContextSize(10000)
	s.SetReservedContext(1000)

	// Should not compress when under limit
	s.tokenCount = 8000
	if s.shouldCompress() {
		t.Error("should not compress when under limit")
	}

	// Should compress when at limit
	s.tokenCount = 9000
	if !s.shouldCompress() {
		t.Error("should compress when at limit")
	}

	// Should compress when over limit
	s.tokenCount = 9500
	if !s.shouldCompress() {
		t.Error("should compress when over limit")
	}
}

func TestSoul_MaybeCompressHistory(t *testing.T) {
	// Mock server that returns a summary response
	server := mockLLMServer(t, []llm.ChatResponse{
		textResponse("Summary of conversation"),
	})
	defer server.Close()

	rt := NewRuntime(t.TempDir(), false)
	rt.LLMClient = llm.NewClient(llm.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10 * time.Second,
	})
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Set small context size to trigger compression
	s.SetMaxContextSize(100)
	s.SetReservedContext(10)
	s.tokenCount = 95 // Above threshold

	// Add 6 messages to history (more than 4 to trigger compression)
	s.llmHistory = []llm.Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "resp1"},
		{Role: "user", Content: "msg2"},
		{Role: "assistant", Content: "resp2"},
		{Role: "user", Content: "msg3"},
		{Role: "assistant", Content: "resp3"},
	}

	err := s.maybeCompressHistory(context.Background(), rt.LLMClient)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	// After compression: summary + last 2 messages
	if len(s.llmHistory) != 3 {
		t.Errorf("expected 3 history entries (summary + 2 kept), got %d", len(s.llmHistory))
	}

	// First message should be the summary
	if s.llmHistory[0].Role != "system" {
		t.Errorf("first message should be system summary, got %s", s.llmHistory[0].Role)
	}
	if !strings.Contains(s.llmHistory[0].Content, "Summary of conversation") {
		t.Errorf("unexpected summary content: %s", s.llmHistory[0].Content)
	}

	// Last 2 messages should be preserved
	if s.llmHistory[1].Content != "msg3" {
		t.Errorf("second message should be 'msg3', got %s", s.llmHistory[1].Content)
	}
	if s.llmHistory[2].Content != "resp3" {
		t.Errorf("third message should be 'resp3', got %s", s.llmHistory[2].Content)
	}
}

func TestSoul_MaybeCompressHistory_NotNeeded(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Set token count below threshold
	s.SetMaxContextSize(10000)
	s.SetReservedContext(1000)
	s.tokenCount = 1000

	// Add some messages
	s.llmHistory = []llm.Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "resp1"},
	}

	// Should not compress when under limit
	err := s.maybeCompressHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("should not error when not compressing: %v", err)
	}

	// History should remain unchanged
	if len(s.llmHistory) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(s.llmHistory))
	}
}

func TestSoul_MaybeCompressHistory_TooFewMessages(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		textResponse("Summary"),
	})
	defer server.Close()

	rt := NewRuntime(t.TempDir(), false)
	rt.LLMClient = llm.NewClient(llm.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10 * time.Second,
	})
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	// Set small context size to trigger compression
	s.SetMaxContextSize(100)
	s.SetReservedContext(10)
	s.tokenCount = 95

	// Add only 3 messages (less than 4, should not compress)
	s.llmHistory = []llm.Message{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "resp1"},
		{Role: "user", Content: "msg2"},
	}

	err := s.maybeCompressHistory(context.Background(), rt.LLMClient)
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// History should remain unchanged (too few messages)
	if len(s.llmHistory) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(s.llmHistory))
	}
}

func TestSoul_GetTokenCount(t *testing.T) {
	rt := NewRuntime(t.TempDir(), false)
	agent := NewAgent("test", "", rt)
	ctx := NewContext("")
	s := NewSoul(agent, ctx)

	if s.GetTokenCount() != 0 {
		t.Errorf("expected initial token count 0, got %d", s.GetTokenCount())
	}

	s.tokenCount = 500
	if s.GetTokenCount() != 500 {
		t.Errorf("expected token count 500, got %d", s.GetTokenCount())
	}
}

func TestEstimateTokens(t *testing.T) {
	// ~4 characters per token
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefghijklmnop", 4},
		{"This is a longer text that should be estimated correctly.", 14},
	}

	for _, tt := range tests {
		got := estimateTokens(tt.text)
		if got != tt.expected {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.expected)
		}
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}

	// Each message: content tokens + 4 overhead
	// "hello" = 1 token, "world" = 1 token
	// Total = 1 + 4 + 1 + 4 = 10
	got := estimateMessagesTokens(messages)
	expected := 10
	if got != expected {
		t.Errorf("estimateMessagesTokens() = %d, want %d", got, expected)
	}
}

func TestSoul_ProcessWithLLM_TokenTracking(t *testing.T) {
	server := mockLLMServer(t, []llm.ChatResponse{
		{
			ID:    "resp-1",
			Model: "test",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      llm.Message `json:"message"`
				Delta        llm.Message `json:"delta"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      llm.Message{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
	})
	defer server.Close()

	s := setupSoul(t, server)
	s.OnMessage = func(msg wire.Message) {}

	userMsg := testMsg(wire.MessageTypeUserInput, "hello")
	err := s.processWithLLM(context.Background(), userMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Token count should be updated from API response
	if s.tokenCount != 150 {
		t.Errorf("expected token count 150, got %d", s.tokenCount)
	}
}
