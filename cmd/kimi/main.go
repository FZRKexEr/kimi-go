// Package main provides the entry point for kimi-go CLI.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"kimi-go/internal/approval"
	"kimi-go/internal/config"
	"kimi-go/internal/llm"
	"kimi-go/internal/session"
	"kimi-go/internal/soul"
	"kimi-go/internal/tools"
	"kimi-go/internal/ui"
	"kimi-go/internal/wire"
)

// defaultLogger 是默认的日志实现
type defaultLogger struct{}

func (l *defaultLogger) Debug(format string, args ...interface{}) {
	// Debug 日志在静默模式下不输出
	// 可以通过设置环境变量 KIMI_DEBUG=1 来开启
	if os.Getenv("KIMI_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func (l *defaultLogger) Info(format string, args ...interface{}) {
	// Info 日志只在交互模式下输出
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		fmt.Fprintf(os.Stderr, "[INFO] "+format+"\n", args...)
	}
}

func (l *defaultLogger) Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", args...)
}

// buildSystemPrompt generates a dynamic system prompt with runtime context.
func buildSystemPrompt(workDir string) string {
	var b strings.Builder

	b.WriteString(`You are Kimi, an interactive AI coding agent running on the user's computer.

Your primary goal is to help the user with programming tasks safely and efficiently, leveraging available tools when needed.

# Tool Use

You have access to the following tools:

- **shell**: Execute shell commands in the working directory. Use this for running builds, tests, git operations, package management, file searching, and any command-line tasks.
- **file**: Perform file operations including read, write, list, delete, and exists checks. Use this for reading source code, writing new files, listing directory contents, and managing files.

When handling the user's request, call available tools to accomplish the task. You may output multiple tool calls in a single response. If you anticipate making multiple non-interfering tool calls, make them in parallel to improve efficiency.

After tool calls return results, determine your next action: continue working, report completion/failure, or ask for clarification.

When responding, use the SAME language as the user unless explicitly instructed otherwise.

# Coding Guidelines

When building something from scratch:
- Understand the user's requirements. Ask for clarification if anything is unclear.
- Design the architecture before writing code.
- Write code in a modular and maintainable way.

When working on an existing codebase:
- Understand the codebase and requirements first. Identify the ultimate goal.
- For bug fixes: check error logs or failed tests, scan the codebase to find root cause, implement a fix. Ensure any mentioned failing tests pass after changes.
- For features: design the architecture, write modular code with minimal intrusion to existing code. Add tests if the project already has tests.
- For refactoring: update all call sites if interfaces change. Do NOT change existing logic especially in tests — only fix errors caused by interface changes.
- Make MINIMAL changes to achieve the goal. Follow the coding style of existing code.

DO NOT run git commit, git push, git reset, git rebase or other git mutations unless explicitly asked. Ask for confirmation before any git mutation.

# Working Environment

`)

	// OS info
	b.WriteString("## Operating System\n\n")
	b.WriteString(fmt.Sprintf("The operating system is `%s/%s`. ", runtime.GOOS, runtime.GOARCH))
	b.WriteString("This is NOT a sandbox — actions immediately affect the user's system. Be cautious. ")
	b.WriteString("Unless explicitly instructed, do not access files outside the working directory.\n\n")

	// Date/time
	b.WriteString("## Date and Time\n\n")
	b.WriteString(fmt.Sprintf("Current date and time: `%s`. ", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("Use this as reference when needed. For exact time, use the shell tool.\n\n")

	// Working directory
	b.WriteString("## Working Directory\n\n")
	b.WriteString(fmt.Sprintf("The working directory is `%s`. ", workDir))
	b.WriteString("This is the project root. File operations use relative paths from here. ")
	b.WriteString("For tool parameters that require absolute paths, use the full path.\n\n")

	// Directory listing
	b.WriteString("Directory listing:\n\n```\n")
	if entries, err := os.ReadDir(workDir); err == nil {
		for _, e := range entries {
			prefix := "  "
			if e.IsDir() {
				prefix = "d "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, e.Name()))
		}
	} else {
		b.WriteString("(unable to list directory)\n")
	}
	b.WriteString("```\n\n")

	// AGENTS.md
	b.WriteString("# Project Information\n\n")
	agentsMD := filepath.Join(workDir, "AGENTS.md")
	if data, err := os.ReadFile(agentsMD); err == nil && len(data) > 0 {
		b.WriteString("The project `AGENTS.md`:\n\n```\n")
		content := string(data)
		if len(content) > 4000 {
			content = content[:4000] + "\n... (truncated)"
		}
		b.WriteString(content)
		b.WriteString("\n```\n\n")
	} else {
		readmePath := filepath.Join(workDir, "README.md")
		if data, err := os.ReadFile(readmePath); err == nil && len(data) > 0 {
			b.WriteString("The project `README.md`:\n\n```\n")
			content := string(data)
			if len(content) > 4000 {
				content = content[:4000] + "\n... (truncated)"
			}
			b.WriteString(content)
			b.WriteString("\n```\n\n")
		} else {
			b.WriteString("No AGENTS.md or README.md found. Explore the project structure as needed.\n\n")
		}
	}

	// Reminders
	b.WriteString(`# Reminders

- Be HELPFUL, CONCISE, and ACCURATE.
- Never diverge from the task requirements. Stay on track.
- Make minimal changes — do not over-engineer.
- Try your best to avoid hallucination. Verify facts with tools when possible.
- Think before you act. Do not give up too early.
- Keep it simple.
`)

	return b.String()
}

func main() {
	var (
		configPath = flag.String("config", "", "Path to config file")
		workDir    = flag.String("work-dir", "", "Working directory")
		sessionID  = flag.String("session", "", "Session ID to continue")
		yolo       = flag.Bool("yolo", false, "Auto-approve all actions")
		version    = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *version {
		fmt.Println("kimi-go v0.1.0")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create or continue session
	var sess *session.Session
	if *sessionID != "" {
		sess, err = session.Continue(*sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error continuing session: %v\n", err)
			os.Exit(1)
		}
	} else {
		sess, err = session.Create(*workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
			os.Exit(1)
		}
		if err := sess.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving session: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Session: %s\n", sess.ID)
	fmt.Printf("WorkDir: %s\n", sess.WorkDir)

	// Create runtime
	rt := soul.NewRuntime(sess.WorkDir, *yolo)
	rt.MaxSteps = cfg.LoopControl.MaxStepsPerTurn
	rt.MaxRetries = cfg.LoopControl.MaxRetriesPerStep

	// Create and inject LLM client
	baseURL := os.Getenv("OPENAI_BASE_URL")
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := os.Getenv("OPENAI_MODEL")

	// Fallback to config if env vars not set
	if baseURL == "" || apiKey == "" || model == "" {
		if provider, ok := cfg.GetDefaultProvider(); ok {
			if baseURL == "" {
				baseURL = provider.BaseURL
			}
			if apiKey == "" {
				apiKey, _ = provider.GetAPIKey()
			}
			if model == "" {
				model = cfg.DefaultModel
			}
		}
	}

	if baseURL != "" && apiKey != "" && model != "" {
		// 创建基础 LLM 客户端
		llmClient := llm.NewClient(llm.Config{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
		})

		// 创建带重试功能的客户端
		var retryCfg *llm.RetryConfig
		if provider, ok := cfg.GetDefaultProvider(); ok && provider.Retry != nil {
			retryCfg = llm.RetryConfigFromProvider(&provider)
		} else {
			retryCfg = llm.DefaultRetryConfig()
		}
		// 从 LoopControl 覆盖最大重试次数
		if cfg.LoopControl.MaxRetriesPerStep > 0 {
			retryCfg.MaxRetries = cfg.LoopControl.MaxRetriesPerStep
		}

		retryClient := llm.NewRetryableClient(llmClient, retryCfg, &defaultLogger{})
		rt.LLMClient = retryClient
		fmt.Printf("LLM: %s @ %s (retries: %d)\n", model, baseURL, retryCfg.MaxRetries)
	} else {
		fmt.Println("Warning: LLM not configured. Set OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL env vars.")
	}

	// Register tools
	shellTool := tools.NewShellTool(sess.WorkDir, 0)
	fileTool := tools.NewFileTool(sess.WorkDir)

	if err := rt.RegisterTool(shellTool); err != nil {
		fmt.Fprintf(os.Stderr, "Error registering shell tool: %v\n", err)
		os.Exit(1)
	}
	if err := rt.RegisterTool(fileTool); err != nil {
		fmt.Fprintf(os.Stderr, "Error registering file tool: %v\n", err)
		os.Exit(1)
	}

	// Create agent with dynamic system prompt
	agent := soul.NewAgent("kimi", buildSystemPrompt(sess.WorkDir), rt)
	agent.AddTool("shell")
	agent.AddTool("file")

	// Create context
	ctx := soul.NewContext(sess.ContextFile)
	if err := ctx.Restore(); err != nil {
		fmt.Fprintf(os.Stderr, "Error restoring context: %v\n", err)
		os.Exit(1)
	}

	// Create soul
	soulInstance := soul.NewSoul(agent, ctx)

	// Create context for soul
	soulCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Detect if stdin is a TTY to decide TUI vs plain REPL mode
	isTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

	if isTTY {
		// ── TUI mode ──
		eventCh := make(chan tea.Msg, 100)

		soulInstance.OnMessage = func(msg wire.Message) {
			eventCh <- ui.SoulMessageMsg{Message: msg}
		}
		soulInstance.OnToolCall = func(tc tools.ToolCall) {
			eventCh <- ui.SoulToolCallMsg{ToolCall: tc}
		}
		soulInstance.OnToolResult = func(tr tools.ToolResult) {
			eventCh <- ui.SoulToolResultMsg{ToolResult: tr}
		}
		soulInstance.OnError = func(err error) {
			eventCh <- ui.SoulErrorMsg{Err: err}
		}
		soulInstance.OnApprovalNeeded = func(req *approval.ApprovalRequest) {
			eventCh <- ui.ApprovalRequestMsg{Request: req, Soul: soulInstance}
		}

		// Bridge DoneCh → eventCh
		go func() {
			for {
				_, ok := <-soulInstance.DoneCh
				if !ok {
					return
				}
				eventCh <- ui.SoulDoneMsg{}
			}
		}()

		// Start soul
		go func() {
			if err := soulInstance.Run(soulCtx); err != nil {
				eventCh <- ui.SoulErrorMsg{Err: err}
			}
		}()

		// Launch TUI
		model := ui.NewModel(soulInstance, eventCh)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// ── Plain REPL mode (non-TTY / pipe input) ──
		soulInstance.OnMessage = func(msg wire.Message) {
			switch msg.Type {
			case wire.MessageTypeAssistant:
				for _, part := range msg.Content {
					if part.Type == "text" {
						fmt.Printf("\nAssistant: %s\n", part.Text)
					}
				}
			case wire.MessageTypeToolCall:
				for _, part := range msg.Content {
					if part.Type == "text" {
						fmt.Printf("\n[Tool Call] %s\n", part.Text)
					}
				}
			case wire.MessageTypeToolResult:
				for _, part := range msg.Content {
					if part.Type == "text" {
						fmt.Printf("[Tool Result] %s\n", part.Text)
					}
				}
			}
		}

		soulInstance.OnError = func(err error) {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		}

		// Set up signal handling
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Start soul in background
		go func() {
			if err := soulInstance.Run(soulCtx); err != nil {
				fmt.Fprintf(os.Stderr, "Soul error: %v\n", err)
			}
		}()

		fmt.Println("\nKimi-Go CLI")
		fmt.Println("Type your message and press Enter. Type 'exit' or 'quit' to quit.")
		fmt.Println()

		scanner := bufio.NewScanner(os.Stdin)
		for {
			select {
			case sig := <-sigCh:
				fmt.Printf("\nReceived signal: %v\n", sig)
				soulInstance.Cancel()
				return
			default:
			}

			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}
			input := strings.TrimSpace(scanner.Text())

			if input == "exit" || input == "quit" {
				fmt.Println("Goodbye!")
				return
			}

			if input == "" {
				continue
			}

			msg := wire.NewTextMessage(wire.MessageTypeUserInput, input)
			if err := soulInstance.SendMessage(*msg); err != nil {
				fmt.Fprintf(os.Stderr, "Error sending message: %v\n", err)
				continue
			}

			select {
			case <-soulInstance.DoneCh:
				// Processing done
			case <-time.After(120 * time.Second):
				fmt.Fprintln(os.Stderr, "Timeout waiting for response")
			}
		}
	}
}
