// Integration tests for LLM client
// These tests use the real API endpoint
// Set environment variables to run:
//
//	OPENAI_API_KEY=your-key go test -v ./llm -run Integration
package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Test configuration - using the provided real API endpoint
// To run these tests:
//
//	export OPENAI_API_KEY="your-api-key"
//	export OPENAI_BASE_URL="https://ark.cn-beijing.volces.com/api/v3"
//	export OPENAI_MODEL="your-model-or-endpoint-id"
//	go test -v ./llm -run Integration
const (
	defaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	defaultModel   = "" // Will be read from env var
)

func getTestConfig(t *testing.T) (baseURL, apiKey, model string) {
	// Get from environment variables
	baseURL = os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	apiKey = os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set")
	}

	model = os.Getenv("OPENAI_MODEL")
	if model == "" {
		t.Skip("Skipping integration test: OPENAI_MODEL not set (e.g., 'gpt-4' or your endpoint ID)")
	}

	t.Logf("Using baseURL: %s", baseURL)
	t.Logf("Using model: %s", model)
	return
}

// TestBasicChat tests a simple chat completion
func TestIntegration_BasicChat(t *testing.T) {
	baseURL, apiKey, model := getTestConfig(t)

	client := NewClient(Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: 60 * time.Second,
	})

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Say hello in one word."},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Fatal("Empty response content")
	}

	t.Logf("Response: %s", content)
	t.Logf("Usage: Prompt=%d, Completion=%d, Total=%d",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens)
}

// TestStreamingChat tests streaming chat completion
func TestIntegration_StreamingChat(t *testing.T) {
	baseURL, apiKey, model := getTestConfig(t)

	client := NewClient(Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: 60 * time.Second,
	})

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant. Be concise."},
		{Role: "user", Content: "Count from 1 to 3."},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	responseChan, errorChan := client.ChatStream(ctx, messages)

	var fullContent strings.Builder
	done := false

	for !done {
		select {
		case chunk, ok := <-responseChan:
			if !ok {
				done = true
				break
			}
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					fullContent.WriteString(content)
					t.Logf("Chunk: %s", content)
				}
			}
		case err := <-errorChan:
			if err != nil {
				t.Fatalf("Stream error: %v", err)
			}
			done = true
		}
	}

	finalContent := fullContent.String()
	if finalContent == "" {
		t.Fatal("Empty streaming response")
	}

	t.Logf("Full response: %s", finalContent)
}

// TestErrorHandling tests error handling for invalid requests
func TestIntegration_ErrorHandling(t *testing.T) {
	baseURL, _, model := getTestConfig(t)

	tests := []struct {
		name       string
		apiKey     string
		wantErr    bool
		errContain string
	}{
		{
			name:       "invalid API key format",
			apiKey:     "invalid-key-format",
			wantErr:    true,
			errContain: "401", // Expect 401 Unauthorized
		},
		{
			name:       "empty API key",
			apiKey:     "",
			wantErr:    true,
			errContain: "400", // Volcengine returns 400 for empty auth header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(Config{
				BaseURL: baseURL,
				APIKey:  tt.apiKey,
				Model:   model,
				Timeout: 10 * time.Second,
			})

			messages := []Message{
				{Role: "user", Content: "Hello"},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := client.Chat(ctx, messages)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContain)
					return
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContain, err)
				}
				t.Logf("Got expected error: %v", err)
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestClientConfiguration tests different client configurations
func TestIntegration_ClientConfiguration(t *testing.T) {
	baseURL, apiKey, model := getTestConfig(t)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "default timeout",
			config: Config{
				BaseURL: baseURL,
				APIKey:  apiKey,
				Model:   model,
			},
			wantErr: false,
		},
		{
			name: "custom timeout",
			config: Config{
				BaseURL: baseURL,
				APIKey:  apiKey,
				Model:   model,
				Timeout: 30 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)

			messages := []Message{
				{Role: "user", Content: "Say 'test' and nothing else."},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resp, err := client.Chat(ctx, messages)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Chat() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && len(resp.Choices) > 0 {
				t.Logf("Response: %s", resp.Choices[0].Message.Content)
			}
		})
	}
}
