package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockServer creates a test server that returns a predefined response.
func mockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10 * time.Second,
	})
	return server, client
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	client := NewClient(Config{
		BaseURL: "http://localhost",
		APIKey:  "key",
		Model:   "model",
	})
	if client.httpClient.Timeout != 120*time.Second {
		t.Errorf("expected 120s default timeout, got %v", client.httpClient.Timeout)
	}
}

func TestNewClient_CustomTimeout(t *testing.T) {
	client := NewClient(Config{
		BaseURL: "http://localhost",
		APIKey:  "key",
		Model:   "model",
		Timeout: 30 * time.Second,
	})
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", client.httpClient.Timeout)
	}
}

func TestChat_Success(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		// Verify request body
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %q", req.Model)
		}
		if req.Stream {
			t.Error("Chat should not use streaming")
		}

		resp := ChatResponse{
			ID:    "resp-1",
			Model: "test-model",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	resp, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", resp.Choices[0].Message.Content)
	}
}

func TestChat_HTTPError(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	})
	defer server.Close()

	_, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401, got: %v", err)
	}
}

func TestChat_InvalidResponseJSON(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not valid json`)
	})
	defer server.Close()

	_, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestChat_ContextCancellation(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Chat(ctx, []Message{
		{Role: "user", Content: "Hi"},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestChatWithTools_Success(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify tools are passed
		if len(req.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(req.Tools))
		}
		if req.Tools[0].Function.Name != "shell" {
			t.Errorf("expected tool name 'shell', got %q", req.Tools[0].Function.Name)
		}

		resp := ChatResponse{
			ID:    "resp-1",
			Model: "test-model",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "I'll run that for you."},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	tools := []ToolDef{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "shell",
				Description: "Execute shell commands",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
			},
		},
	}

	resp, err := client.ChatWithTools(context.Background(), []Message{
		{Role: "user", Content: "list files"},
	}, tools)
	if err != nil {
		t.Fatalf("ChatWithTools failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "I'll run that for you." {
		t.Errorf("unexpected response: %q", resp.Choices[0].Message.Content)
	}
}

func TestChatWithTools_ToolCallResponse(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:    "resp-1",
			Model: "test-model",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: Message{
						Role: "assistant",
						ToolCalls: []ToolCallInfo{
							{
								ID:   "call_1",
								Type: "function",
								Function: FunctionCall{
									Name:      "shell",
									Arguments: `{"command":"ls"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	resp, err := client.ChatWithTools(context.Background(), []Message{
		{Role: "user", Content: "list files"},
	}, nil)
	if err != nil {
		t.Fatalf("ChatWithTools failed: %v", err)
	}

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "shell" {
		t.Errorf("expected tool name 'shell', got %q", msg.ToolCalls[0].Function.Name)
	}
	if msg.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected call ID 'call_1', got %q", msg.ToolCalls[0].ID)
	}
}

func TestChatWithTools_NilTools(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Tools != nil {
			t.Error("tools should be nil/omitted when not provided")
		}

		resp := ChatResponse{
			ID: "resp-1",
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				Delta        Message `json:"delta"`
				FinishReason string  `json:"finish_reason"`
			}{
				{Message: Message{Role: "assistant", Content: "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	_, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestChatStream_Success(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("ChatStream should set stream=true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`{"id":"1","choices":[{"delta":{"content":"Hello"}}]}`,
			`{"id":"2","choices":[{"delta":{"content":" world"}}]}`,
		}
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()

	respCh, errCh := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})

	var content strings.Builder
	for chunk := range respCh {
		if len(chunk.Choices) > 0 {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	if err := <-errCh; err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if content.String() != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", content.String())
	}
}

func TestChatStream_HTTPError(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	})
	defer server.Close()

	_, errCh := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})

	err := <-errCh
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestChatStream_MalformedChunks(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: not-json\n\n")
		fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()

	respCh, errCh := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})

	count := 0
	for range respCh {
		count++
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Malformed chunk should be skipped, only valid chunk received
	if count != 1 {
		t.Errorf("expected 1 valid chunk (skipping malformed), got %d", count)
	}
}

func TestMessage_JSON_WithToolCalls(t *testing.T) {
	msg := Message{
		Role: "assistant",
		ToolCalls: []ToolCallInfo{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "shell",
					Arguments: `{"command":"ls"}`,
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call after round-trip, got %d", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Function.Name != "shell" {
		t.Errorf("expected 'shell', got %q", decoded.ToolCalls[0].Function.Name)
	}
}

func TestMessage_JSON_WithToolCallID(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    "result data",
		ToolCallID: "call_1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Message
	json.Unmarshal(data, &decoded)

	if decoded.ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id 'call_1', got %q", decoded.ToolCallID)
	}
}

func TestMessage_JSON_OmitEmpty(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "hello",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	str := string(data)
	if strings.Contains(str, "tool_calls") {
		t.Error("tool_calls should be omitted when empty")
	}
	if strings.Contains(str, "tool_call_id") {
		t.Error("tool_call_id should be omitted when empty")
	}
}

func TestChatStream_EmptyLines(t *testing.T) {
	server, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// SSE often has empty lines, comments, etc.
		fmt.Fprint(w, "\n")
		fmt.Fprint(w, ": comment\n")
		fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fmt.Fprint(w, "\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()

	respCh, errCh := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "Hi"},
	})

	count := 0
	for range respCh {
		count++
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 chunk, got %d", count)
	}
}
