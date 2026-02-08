// Package llm provides LLM client implementations.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client represents an LLM client.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// Config represents the client configuration.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// NewClient creates a new LLM client.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ToolDef represents a tool definition for the LLM.
type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef represents a function definition within a tool.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCallInfo represents a tool call returned by the LLM.
type ToolCallInfo struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function name and arguments in a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message represents a chat message.
type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []ToolCallInfo `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []ToolDef `json:"tools,omitempty"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatWithTools sends a chat completion request with tool definitions.
func (c *Client) ChatWithTools(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}

	return c.sendRequest(ctx, reqBody)
}

// Chat sends a chat completion request.
func (c *Client) Chat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	return c.ChatWithTools(ctx, messages, nil)
}

// Ensure Client implements the interface
var _ interface {
	Chat(ctx context.Context, messages []Message) (*ChatResponse, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error)
} = (*Client)(nil)

// ChatStream sends a streaming chat completion request.
func (c *Client) ChatStream(ctx context.Context, messages []Message) (<-chan ChatResponse, <-chan error) {
	return c.ChatStreamWithTools(ctx, messages, nil)
}

// ChatStreamWithTools sends a streaming chat completion request with tool definitions.
func (c *Client) ChatStreamWithTools(ctx context.Context, messages []Message, tools []ToolDef) (<-chan ChatResponse, <-chan error) {
	responseChan := make(chan ChatResponse)
	errorChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errorChan)

		reqBody := ChatRequest{
			Model:    c.model,
			Messages: messages,
			Stream:   true,
			Tools:    tools,
		}

		if err := c.sendStreamRequest(ctx, reqBody, responseChan); err != nil {
			errorChan <- err
		}
	}()

	return responseChan, errorChan
}

// sendRequest sends a non-streaming request.
func (c *Client) sendRequest(ctx context.Context, reqBody ChatRequest) (*ChatResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			RawBody:    string(body),
		}
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// sendStreamRequest sends a streaming request.
func (c *Client) sendStreamRequest(ctx context.Context, reqBody ChatRequest, responseChan chan<- ChatResponse) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			RawBody:    string(body),
		}
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		responseChan <- chunk
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}
