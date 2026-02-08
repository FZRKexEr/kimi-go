// Package benchmark provides an evaluation framework for the kimi-go agent.
// It runs predefined scenarios against a real LLM and measures capabilities.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"kimi-go/internal/llm"
	"kimi-go/internal/soul"
	"kimi-go/internal/tools"
	"kimi-go/internal/wire"
)

// Case defines a benchmark scenario.
type Case struct {
	Name        string        // Short name
	Description string        // What this tests
	Messages    []string      // Sequence of user messages to send
	Assertions  []Assertion   // Conditions to check
	Timeout     time.Duration // Per-case timeout
}

// Assertion defines a condition to verify after a case runs.
type Assertion struct {
	Type  AssertionType
	Value string // Expected value (meaning depends on Type)
}

// AssertionType enumerates the kinds of assertions.
type AssertionType string

const (
	// AssertResponseContains checks that any assistant response contains Value.
	AssertResponseContains AssertionType = "response_contains"
	// AssertToolCalled checks that a tool with name Value was called.
	AssertToolCalled AssertionType = "tool_called"
	// AssertNoError checks that no errors occurred.
	AssertNoError AssertionType = "no_error"
	// AssertMinResponses checks that at least Value (as int) assistant responses were received.
	AssertMinResponses AssertionType = "min_responses"
)

// Result captures the outcome of a single benchmark case.
type Result struct {
	Case         string        `json:"case"`
	Passed       bool          `json:"passed"`
	Duration     time.Duration `json:"duration_ms"`
	ToolCalls    int           `json:"tool_calls"`
	LLMCalls     int           `json:"llm_calls"`
	Responses    int           `json:"responses"`
	Errors       []string      `json:"errors,omitempty"`
	FailedChecks []string      `json:"failed_checks,omitempty"`
}

// Report is the full benchmark output.
type Report struct {
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	BaseURL   string    `json:"base_url"`
	Results   []Result  `json:"results"`
	Summary   Summary   `json:"summary"`
}

// Summary aggregates benchmark statistics.
type Summary struct {
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	TotalTime time.Duration `json:"total_time_ms"`
	AvgTime   time.Duration `json:"avg_time_ms"`
	PassRate  float64       `json:"pass_rate"`
	ToolCalls int           `json:"total_tool_calls"`
	LLMCalls  int           `json:"total_llm_calls"`
}

// recorder captures events during a benchmark run.
type recorder struct {
	messages    []wire.Message
	toolCalls   []tools.ToolCall
	toolResults []tools.ToolResult
	errors      []error
}

// Runner executes benchmark cases.
type Runner struct {
	BaseURL string
	APIKey  string
	Model   string
}

// NewRunnerFromEnv creates a Runner from environment variables.
// Returns nil if required vars are not set.
func NewRunnerFromEnv() *Runner {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := os.Getenv("OPENAI_MODEL")

	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	return &Runner{BaseURL: baseURL, APIKey: apiKey, Model: model}
}

// Run executes all cases and returns a report.
func (r *Runner) Run(cases []Case) *Report {
	report := &Report{
		Timestamp: time.Now(),
		Model:     r.Model,
		BaseURL:   r.BaseURL,
		Results:   make([]Result, 0, len(cases)),
	}

	for _, c := range cases {
		result := r.runCase(c)
		report.Results = append(report.Results, result)
	}

	// Compute summary
	var totalTime time.Duration
	for _, res := range report.Results {
		report.Summary.Total++
		if res.Passed {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
		totalTime += res.Duration
		report.Summary.ToolCalls += res.ToolCalls
		report.Summary.LLMCalls += res.LLMCalls
	}
	report.Summary.TotalTime = totalTime
	if report.Summary.Total > 0 {
		report.Summary.AvgTime = totalTime / time.Duration(report.Summary.Total)
		report.Summary.PassRate = float64(report.Summary.Passed) / float64(report.Summary.Total) * 100
	}

	return report
}

func (r *Runner) runCase(c Case) Result {
	result := Result{Case: c.Name}
	start := time.Now()

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	// Create isolated environment
	workDir, err := os.MkdirTemp("", "kimi-bench-*")
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create temp dir: %v", err))
		result.Duration = time.Since(start)
		return result
	}
	defer os.RemoveAll(workDir)

	// Set up soul
	runtime := soul.NewRuntime(workDir, true)
	runtime.LLMClient = llm.NewClient(llm.Config{
		BaseURL: r.BaseURL,
		APIKey:  r.APIKey,
		Model:   r.Model,
		Timeout: timeout,
	})
	runtime.MaxSteps = 20

	shellTool := tools.NewShellTool(workDir, 30*time.Second)
	fileTool := tools.NewFileTool(workDir)
	runtime.RegisterTool(shellTool)
	runtime.RegisterTool(fileTool)

	agent := soul.NewAgent("benchmark", "You are a helpful assistant. Use tools when needed. Be concise.", runtime)
	agent.AddTool("shell")
	agent.AddTool("file")

	soulCtx := soul.NewContext("")
	s := soul.NewSoul(agent, soulCtx)

	// Set up recording
	rec := &recorder{}
	s.OnMessage = func(msg wire.Message) {
		rec.messages = append(rec.messages, msg)
	}
	s.OnToolCall = func(tc tools.ToolCall) {
		rec.toolCalls = append(rec.toolCalls, tc)
	}
	s.OnToolResult = func(tr tools.ToolResult) {
		rec.toolResults = append(rec.toolResults, tr)
	}
	s.OnError = func(err error) {
		rec.errors = append(rec.errors, err)
	}

	// Run soul
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go s.Run(ctx)
	time.Sleep(50 * time.Millisecond) // Wait for soul to start

	// Send messages
	for _, input := range c.Messages {
		msg := wire.NewTextMessage(wire.MessageTypeUserInput, input)
		if err := s.SendMessage(*msg); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("send failed: %v", err))
			break
		}

		// Wait for processing
		select {
		case <-s.DoneCh:
		case <-ctx.Done():
			result.Errors = append(result.Errors, "timeout")
			break
		}
	}

	result.Duration = time.Since(start)
	result.ToolCalls = len(rec.toolCalls)

	// Count LLM calls (assistant messages + tool_call messages from LLM perspective)
	for _, m := range rec.messages {
		if m.Type == wire.MessageTypeAssistant {
			result.Responses++
			result.LLMCalls++
		}
		if m.Type == wire.MessageTypeToolCall {
			result.LLMCalls++ // Each tool call round is an LLM call
		}
	}

	// Convert errors
	for _, e := range rec.errors {
		result.Errors = append(result.Errors, e.Error())
	}

	// Check assertions
	result.Passed = true
	for _, a := range c.Assertions {
		if err := checkAssertion(a, rec, &result); err != nil {
			result.FailedChecks = append(result.FailedChecks, err.Error())
			result.Passed = false
		}
	}

	return result
}

func checkAssertion(a Assertion, rec *recorder, result *Result) error {
	switch a.Type {
	case AssertResponseContains:
		for _, m := range rec.messages {
			if m.Type == wire.MessageTypeAssistant {
				for _, p := range m.Content {
					if strings.Contains(p.Text, a.Value) {
						return nil
					}
				}
			}
		}
		return fmt.Errorf("no response contains %q", a.Value)

	case AssertToolCalled:
		for _, tc := range rec.toolCalls {
			if tc.Name == a.Value {
				return nil
			}
		}
		return fmt.Errorf("tool %q was not called", a.Value)

	case AssertNoError:
		if len(rec.errors) > 0 || len(result.Errors) > 0 {
			allErrs := append(result.Errors, errorsToStrings(rec.errors)...)
			return fmt.Errorf("errors occurred: %v", allErrs)
		}
		return nil

	case AssertMinResponses:
		var expected int
		fmt.Sscanf(a.Value, "%d", &expected)
		if result.Responses < expected {
			return fmt.Errorf("expected at least %d responses, got %d", expected, result.Responses)
		}
		return nil

	default:
		return fmt.Errorf("unknown assertion type: %s", a.Type)
	}
}

func errorsToStrings(errs []error) []string {
	s := make([]string, len(errs))
	for i, e := range errs {
		s[i] = e.Error()
	}
	return s
}

// PrintReport prints a formatted benchmark report.
func PrintReport(report *Report) {
	fmt.Println("============================================")
	fmt.Println("  Kimi-Go Agent Benchmark Report")
	fmt.Println("============================================")
	fmt.Printf("  Model:     %s\n", report.Model)
	fmt.Printf("  Endpoint:  %s\n", report.BaseURL)
	fmt.Printf("  Time:      %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println()

	fmt.Println("  Results:")
	fmt.Println("  --------")
	for _, r := range report.Results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %-35s %6dms  tools=%d llm=%d\n",
			status, r.Case, r.Duration.Milliseconds(), r.ToolCalls, r.LLMCalls)
		for _, fc := range r.FailedChecks {
			fmt.Printf("         -> %s\n", fc)
		}
		for _, e := range r.Errors {
			fmt.Printf("         !! %s\n", e)
		}
	}

	fmt.Println()
	fmt.Println("  Summary:")
	fmt.Println("  --------")
	fmt.Printf("  Total: %d  Passed: %d  Failed: %d  Rate: %.0f%%\n",
		report.Summary.Total, report.Summary.Passed, report.Summary.Failed, report.Summary.PassRate)
	fmt.Printf("  Total time: %dms  Avg: %dms\n",
		report.Summary.TotalTime.Milliseconds(), report.Summary.AvgTime.Milliseconds())
	fmt.Printf("  Tool calls: %d  LLM calls: %d\n",
		report.Summary.ToolCalls, report.Summary.LLMCalls)
	fmt.Println("============================================")
}

// SaveReport saves a report to a JSON file.
func SaveReport(report *Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
