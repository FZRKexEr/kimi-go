// Package soul provides the core agent implementation.
package soul

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"kimi-go/internal/llm"
	"kimi-go/internal/tools"
	"kimi-go/internal/wire"
)

// LLMClient 定义 LLM 客户端接口
type LLMClient interface {
	Chat(ctx context.Context, messages []llm.Message) (*llm.ChatResponse, error)
	ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*llm.ChatResponse, error)
}

// Runtime provides the execution environment for the agent.
type Runtime struct {
	WorkDir    string
	Config     map[string]any
	Tools      *tools.ToolSet
	LLMClient  LLMClient
	YOLO       bool // Auto-approve mode
	MaxSteps   int
	MaxRetries int
}

// NewRuntime creates a new runtime.
func NewRuntime(workDir string, yolo bool) *Runtime {
	return &Runtime{
		WorkDir:    workDir,
		Config:     make(map[string]any),
		Tools:      tools.NewToolSet(),
		YOLO:       yolo,
		MaxSteps:   100,
		MaxRetries: 3,
	}
}

// RegisterTool registers a tool with the runtime.
func (r *Runtime) RegisterTool(tool tools.Tool) error {
	return r.Tools.Register(tool)
}

// Agent represents the agent configuration.
type Agent struct {
	Name         string
	SystemPrompt string
	Tools        []string // Tool names
	Runtime      *Runtime
}

// NewAgent creates a new agent.
func NewAgent(name, systemPrompt string, runtime *Runtime) *Agent {
	return &Agent{
		Name:         name,
		SystemPrompt: systemPrompt,
		Tools:        make([]string, 0),
		Runtime:      runtime,
	}
}

// AddTool adds a tool to the agent.
func (a *Agent) AddTool(toolName string) {
	a.Tools = append(a.Tools, toolName)
}

// Soul is the core agent that processes messages and executes tools.
type Soul struct {
	Agent   *Agent
	Context *Context
	runtime *Runtime

	mu       sync.Mutex
	running  bool
	cancelCh chan struct{}
	msgCh    chan wire.Message

	// LLM conversation history (separate from wire context)
	llmHistory []llm.Message

	// DoneCh is closed after each message is fully processed
	DoneCh chan struct{}

	// Handlers
	OnMessage    func(wire.Message)
	OnToolCall   func(tools.ToolCall)
	OnToolResult func(tools.ToolResult)
	OnError      func(error)
}

// NewSoul creates a new Soul instance.
func NewSoul(agent *Agent, ctx *Context) *Soul {
	return &Soul{
		Agent:      agent,
		Context:    ctx,
		runtime:    agent.Runtime,
		cancelCh:   make(chan struct{}),
		msgCh:      make(chan wire.Message, 100),
		llmHistory: make([]llm.Message, 0),
		DoneCh:     make(chan struct{}, 1),
	}
}

// Run starts the soul's main loop.
func (s *Soul) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("soul is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.cancelCh:
			return fmt.Errorf("soul cancelled")
		case msg := <-s.msgCh:
			if err := s.processMessage(ctx, msg); err != nil {
				s.handleError(err)
			}
			// Signal that processing is done
			select {
			case s.DoneCh <- struct{}{}:
			default:
			}
		}
	}
}

// SendMessage sends a message to the soul for processing.
func (s *Soul) SendMessage(msg wire.Message) error {
	select {
	case s.msgCh <- msg:
		return nil
	default:
		return fmt.Errorf("message channel is full")
	}
}

// Cancel cancels the current operation.
func (s *Soul) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.cancelCh:
		// Already closed, recreate
	default:
		close(s.cancelCh)
	}
	s.cancelCh = make(chan struct{})
}

// IsRunning returns whether the soul is running.
func (s *Soul) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// processMessage processes a single message.
func (s *Soul) processMessage(ctx context.Context, msg wire.Message) error {
	switch msg.Type {
	case wire.MessageTypeUserInput:
		return s.handleUserInput(ctx, msg)
	case wire.MessageTypeCancel:
		s.Cancel()
		return nil
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleUserInput handles user input messages.
func (s *Soul) handleUserInput(ctx context.Context, msg wire.Message) error {
	// Add user message to context
	s.Context.AddMessage(msg)

	// Process with LLM and tools
	return s.processWithLLM(ctx, msg)
}

// processWithLLM runs the agent loop: call LLM, execute tools, repeat.
func (s *Soul) processWithLLM(ctx context.Context, userMsg wire.Message) error {
	client := s.runtime.LLMClient
	if client == nil {
		// Fallback: no LLM client, return a static response
		response := wire.Message{
			Type: wire.MessageTypeAssistant,
			Content: []wire.ContentPart{{
				Type: "text",
				Text: "LLM client is not configured. Please set OPENAI_BASE_URL, OPENAI_API_KEY, and OPENAI_MODEL environment variables.",
			}},
			Timestamp: time.Now(),
		}
		s.Context.AddMessage(response)
		if s.OnMessage != nil {
			s.OnMessage(response)
		}
		return nil
	}

	// Extract user text from wire message
	userText := extractText(userMsg)

	// Add user message to LLM history
	s.llmHistory = append(s.llmHistory, llm.Message{
		Role:    "user",
		Content: userText,
	})

	// Build full message list with system prompt
	messages := s.buildLLMMessages()

	// Build tool definitions
	toolDefs := s.buildToolDefs()

	// Agent loop
	for step := 0; step < s.runtime.MaxSteps; step++ {
		resp, err := client.ChatWithTools(ctx, messages, toolDefs)
		if err != nil {
			return fmt.Errorf("LLM request failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("LLM returned no choices")
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message

		// Add assistant message to LLM history
		s.llmHistory = append(s.llmHistory, assistantMsg)
		// Also add to the messages list for next iteration
		messages = append(messages, assistantMsg)

		// Check if the LLM wants to call tools
		if len(assistantMsg.ToolCalls) > 0 {
			for _, tc := range assistantMsg.ToolCalls {
				// Emit tool call event
				toolCall := tools.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(tc.Function.Arguments),
				}
				if s.OnToolCall != nil {
					s.OnToolCall(toolCall)
				}

				// Emit wire message for tool call display
				tcMsg := wire.Message{
					Type: wire.MessageTypeToolCall,
					Content: []wire.ContentPart{{
						Type: "text",
						Text: fmt.Sprintf("Calling tool: %s(%s)", tc.Function.Name, tc.Function.Arguments),
					}},
					Timestamp: time.Now(),
				}
				s.Context.AddMessage(tcMsg)
				if s.OnMessage != nil {
					s.OnMessage(tcMsg)
				}

				// Execute the tool
				result, execErr := s.executeToolCall(ctx, toolCall)
				if execErr != nil {
					return fmt.Errorf("tool execution error: %w", execErr)
				}

				// Emit tool result event
				if s.OnToolResult != nil {
					s.OnToolResult(*result)
				}

				// Build result text
				resultText := result.Result
				if !result.Success {
					resultText = fmt.Sprintf("Error: %s", result.Error)
				}

				// Add tool result to LLM history and messages
				toolResultMsg := llm.Message{
					Role:       "tool",
					Content:    resultText,
					ToolCallID: tc.ID,
				}
				s.llmHistory = append(s.llmHistory, toolResultMsg)
				messages = append(messages, toolResultMsg)

				// Emit wire message for tool result display
				trMsg := wire.Message{
					Type: wire.MessageTypeToolResult,
					Content: []wire.ContentPart{{
						Type: "text",
						Text: resultText,
					}},
					Timestamp: time.Now(),
				}
				s.Context.AddMessage(trMsg)
				if s.OnMessage != nil {
					s.OnMessage(trMsg)
				}
			}
			// Continue the loop to let LLM process tool results
			continue
		}

		// No tool calls — this is the final text response
		responseText := assistantMsg.Content
		if responseText == "" {
			responseText = "(empty response)"
		}

		response := wire.Message{
			Type: wire.MessageTypeAssistant,
			Content: []wire.ContentPart{{
				Type: "text",
				Text: responseText,
			}},
			Timestamp: time.Now(),
		}
		s.Context.AddMessage(response)
		if s.OnMessage != nil {
			s.OnMessage(response)
		}

		// Save context after successful response
		_ = s.Context.Save()
		return nil
	}

	return fmt.Errorf("agent loop exceeded maximum steps (%d)", s.runtime.MaxSteps)
}

// buildLLMMessages constructs the full message list including system prompt.
func (s *Soul) buildLLMMessages() []llm.Message {
	messages := make([]llm.Message, 0, len(s.llmHistory)+1)

	// System prompt
	if s.Agent.SystemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: s.Agent.SystemPrompt,
		})
	}

	// Append conversation history
	messages = append(messages, s.llmHistory...)
	return messages
}

// buildToolDefs converts registered tools into LLM tool definitions.
func (s *Soul) buildToolDefs() []llm.ToolDef {
	infos := s.runtime.Tools.GetToolInfo()
	if len(infos) == 0 {
		return nil
	}

	defs := make([]llm.ToolDef, len(infos))
	for i, info := range infos {
		defs[i] = llm.ToolDef{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        info.Name,
				Description: info.Description,
				Parameters:  info.Parameters,
			},
		}
	}
	return defs
}

// handleError handles errors.
func (s *Soul) handleError(err error) {
	if s.OnError != nil {
		s.OnError(err)
	}
}

// executeToolCall executes a tool call.
func (s *Soul) executeToolCall(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	tool, err := s.runtime.Tools.Get(call.Name)
	if err != nil {
		return &tools.ToolResult{
			CallID:  call.ID,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return &tools.ToolResult{
			CallID:  call.ID,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.Marshal(result)
	return &tools.ToolResult{
		CallID:  call.ID,
		Success: true,
		Result:  string(resultJSON),
	}, nil
}

// extractText extracts text content from a wire message.
func extractText(msg wire.Message) string {
	for _, part := range msg.Content {
		if part.Type == "text" {
			return part.Text
		}
	}
	return ""
}
