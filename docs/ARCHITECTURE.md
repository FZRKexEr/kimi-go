# Kimi-Go 架构设计文档

## 系统概述

Kimi-Go 是基于 OpenAI 兼容 API 的 AI 编程助手 CLI，采用分层架构，核心是 Agent Loop（LLM ↔ Tool 自动循环）。

## 架构层次

```
┌─────────────────────────────────────┐
│  CLI Layer (cmd/kimi)               │
│  - 命令行参数解析                    │
│  - REPL 交互循环                    │
│  - 系统提示词构建                    │
│  - LLM Client 创建与注入            │
├─────────────────────────────────────┤
│  Core Layer (internal/soul)         │
│  - Soul: 消息路由、Agent Loop       │
│  - Agent: 工具配置、系统提示词       │
│  - Runtime: 执行环境、工具注册       │
│  - Context: 对话历史、持久化         │
├─────────────────────────────────────┤
│  LLM Layer (internal/llm)           │
│  - Chat / ChatWithTools / Stream    │
│  - Tool Calling 协议                │
│  - OpenAI 兼容请求/响应            │
├─────────────────────────────────────┤
│  Tool Layer (internal/tools)        │
│  - Tool 接口                        │
│  - ToolSet 注册表                   │
│  - Shell 工具（命令执行）           │
│  - File 工具（文件操作）            │
├─────────────────────────────────────┤
│  Support Layer                      │
│  - Wire (internal/wire): 消息协议   │
│  - Config (internal/config): TOML   │
│  - Session (internal/session): 会话 │
│  - Logger (internal/logger): 日志   │
└─────────────────────────────────────┘
```

## Agent Loop 核心流程

```
processWithLLM():
  1. 将用户消息加入 llmHistory
  2. buildLLMMessages(): [system prompt] + llmHistory
  3. buildToolDefs(): 从 Runtime.Tools 生成 ToolDef 列表
  4. 循环（最多 MaxSteps 轮）:
     a. ChatWithTools(ctx, messages, toolDefs) → resp
     b. 如果 resp 包含 tool_calls:
        - 逐个执行 Tool (executeToolCall)
        - 将 tool result 以 role="tool" 追加到 messages
        - 触发 OnToolCall / OnToolResult 回调
        - continue（回到 step a）
     c. 如果 resp 是纯文本:
        - 触发 OnMessage 回调
        - 保存 Context
        - return
```

## 核心组件

### Soul (`internal/soul/soul.go`)

```go
type Soul struct {
    Agent      *Agent
    Context    *Context
    runtime    *Runtime
    msgCh      chan wire.Message   // 消息队列
    llmHistory []llm.Message      // LLM 对话历史
    DoneCh     chan struct{}       // 处理完成信号
    // 回调
    OnMessage    func(wire.Message)
    OnToolCall   func(tools.ToolCall)
    OnToolResult func(tools.ToolResult)
    OnError      func(error)
}
```

- `Run(ctx)`: 主循环，从 `msgCh` 读取消息
- `SendMessage(msg)`: 投递消息到队列
- `Cancel()`: 取消当前操作（sync.Mutex 保护）
- `processWithLLM()`: Agent Loop 核心

### Runtime (`internal/soul/soul.go`)

```go
type Runtime struct {
    WorkDir    string
    Tools      *tools.ToolSet
    LLMClient  *llm.Client
    YOLO       bool
    MaxSteps   int
    MaxRetries int
}
```

### LLM Client (`internal/llm/client.go`)

```go
type Client struct { ... }

func (c *Client) Chat(ctx, messages) (*ChatResponse, error)
func (c *Client) ChatWithTools(ctx, messages, tools) (*ChatResponse, error)
func (c *Client) ChatStream(ctx, messages, callback) error
```

关键类型：
- `Message`: role + content + tool_calls + tool_call_id
- `ToolDef`: OpenAI function calling 格式
- `ToolCallInfo`: LLM 返回的工具调用信息
- `ChatResponse`: LLM 响应（含 choices）

### Tool 接口 (`internal/tools/tool.go`)

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}
```

已实现：
- **ShellTool**: 执行 shell 命令，支持超时
- **FileTool**: 文件读写删列，路径相对于 workDir

### Wire 协议 (`internal/wire/types.go`)

```go
type Message struct {
    Type      MessageType
    ID        string
    Content   []ContentPart
    Timestamp time.Time
}
```

消息类型：UserInput、Assistant、ToolCall、ToolResult、Error、Cancel

## 数据流

```
用户输入 "列出文件"
  → CLI: wire.NewTextMessage(UserInput, "列出文件")
  → Soul.SendMessage() → msgCh
  → Soul.Run() → processMessage() → handleUserInput()
  → processWithLLM():
      messages = [system, user:"列出文件"]
      toolDefs = [shell, file]
      → LLMClient.ChatWithTools()
      ← assistant: tool_calls=[{shell, ls}]
      → ShellTool.Execute("ls")
      ← result: "file1.go\nfile2.go"
      messages += [assistant(tool_calls), tool(result)]
      → LLMClient.ChatWithTools()
      ← assistant: "当前目录有 file1.go 和 file2.go"
      → OnMessage(assistant)
      → Context.Save()
      → DoneCh ← struct{}{}
  ← CLI: 打印 "Assistant: 当前目录有..."
```

## 系统提示词

`buildSystemPrompt(workDir)` 在 `cmd/kimi/main.go` 中动态生成，包含：

1. 角色定义与工具说明
2. 编码规范（新建/修改/重构）
3. 运行环境（OS、时间、工作目录、目录列表）
4. 项目信息（读取 AGENTS.md 或 README.md）
5. 行为提醒

## 扩展

### 添加新工具

1. 实现 `tools.Tool` 接口
2. 在 `cmd/kimi/main.go` 注册到 Runtime
3. Agent 添加工具名
4. 更新 `buildSystemPrompt()` 中的工具描述

### 支持新 LLM Provider

只需 API 兼容 OpenAI `/v1/chat/completions` 格式（含 tool calling），配置对应的 `OPENAI_BASE_URL` 即可。
