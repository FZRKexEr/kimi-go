# kimi-go vs kimi-cli 核心能力对比

> 基准时间：2026-02-08
> kimi-cli 版本：v1.9.0 (Python)
> kimi-go 版本：v0.1.0 (Go)

## 一、已对齐的能力

| 能力 | kimi-cli | kimi-go | 状态 |
|------|----------|---------|------|
| Agent 循环（LLM → tool → LLM） | `_agent_loop` + `_step` | `processWithLLM` for loop | 对齐 |
| MaxSteps 限制 | 默认 100 | 默认 100 | 对齐 |
| Shell Tool | `sh -c`，超时 5min | `sh -c`，超时 60s (max 300s) | 对齐 |
| File Tool（读写删查） | ReadFile/WriteFile/Glob/Grep | read/write/list/delete/exists | 基本对齐 |
| 系统提示词 | Jinja2 模板，含 OS/时间/目录/AGENTS.md | Go 拼接，含 OS/时间/目录/AGENTS.md | 对齐 |
| 会话持久化 | JSONL context + wire log | JSON context + session file | 对齐 |
| TOML 配置 + 多 Provider | 支持 | 支持 | 对齐 |
| OpenAI 兼容 API 调用 | kosong → ChatProvider | `llm.Client.ChatWithTools` | 对齐 |
| 工具调用结果返回给 LLM | tool_call_id 匹配 | tool_call_id 匹配 | 对齐 |
| Tool 执行失败不中断循环 | ToolResult(is_error=True) → LLM 处理 | ToolResult(Success=false) → LLM 处理 | 对齐 |
| 非 TTY 管道回退 | print mode | isatty 检测 → 纯 REPL | 对齐 |
| TUI 界面 | Rich + Prompt Toolkit | Charm 全家桶（bubbletea） | 对齐 |

## 二、影响 Agent 能力的关键差距

按影响程度从大到小排序。

### P0：必须修复

#### 1. LLM 错误重试

| | kimi-cli | kimi-go |
|---|---|---|
| 实现 | tenacity: 指数退避 + jitter，重试 429/500/502/503/连接超时/空响应，最多 3 次 | 无任何重试。一次失败整个 turn 就挂 |

线上 API 偶尔返回 429/502 是常态，没有重试意味着一个瞬时错误就打断整个对话。

#### 2. Token 计数 + 上下文自动压缩

| | kimi-cli | kimi-go |
|---|---|---|
| Token 计数 | 从 API 响应的 `usage.input_tokens` 持续追踪 | `ChatResponse.Usage` 字段存在但从未读取 |
| 自动压缩 | 检测 `token_count + reserved >= max_context`，调用 LLM 总结历史，保留最后 2 条消息 | 完全没有。历史无限增长，最终爆 context window |

长对话必然超出 context window，要么 API 报错，要么历史被截断丢失关键信息。

### P1：显著提升体验

#### 3. Streaming 响应

| | kimi-cli | kimi-go |
|---|---|---|
| 实现 | 所有 provider 都流式输出，逐 chunk 显示 | `ChatStream` 方法已实现但 agent loop 不使用。等完整响应后一次性显示 |

对于长回复，用户可能等 20-30 秒看到空白 + spinner，体验远不如逐字输出。

#### 4. 工具输出截断（发给 LLM 的）

| | kimi-cli | kimi-go |
|---|---|---|
| 发送给 LLM | 50,000 字符上限，单行 2,000 字符上限 | 无截断，全量发送 |
| 显示给用户 | 分层展示（display vs message） | UI 层截断到 2,000 字符，但 LLM 收到全量 |

一个 `cat` 大文件的结果可能吃掉大量 context window。

#### 5. 工具并行执行

| | kimi-cli | kimi-go |
|---|---|---|
| 实现 | asyncio 并发执行同一轮的多个 tool call | 顺序执行 |

当 LLM 一次返回 3 个工具调用时，kimi-go 串行执行，慢 N 倍。

### P2：安全与规范

#### 6. YOLO / 权限审批

| | kimi-cli | kimi-go |
|---|---|---|
| 实现 | 三层审批：yolo 全通过 / session 级自动批准 / 逐次审批 | `--yolo` flag 存在但无实际效果。所有工具无条件执行 |

## 三、高级功能差距（非核心）

| 功能 | kimi-cli | kimi-go | 影响 |
|------|----------|---------|------|
| Subagent 多代理 | Task tool 派生子代理，共享审批 | 无 | 复杂任务编排能力缺失 |
| 思维链 / Thinking | `with_thinking("high")`，ThinkPart | 无 | 深度推理能力受限 |
| MCP 工具协议 | fastmcp 集成，后台加载 | 无 | 无法接入外部工具生态 |
| Web 搜索/抓取 | SearchWeb + FetchURL | 无 | 无法获取实时信息 |
| Skill 系统 | 多层级发现 + flow 编排 | 无 | 无法扩展自定义工作流 |
| D-Mail 上下文回溯 | 回滚到 checkpoint + 注入消息 | 无 | 无法修正错误探索路径 |
| Glob / Grep 专用工具 | 独立工具，有参数限制 | 无（只能通过 shell） | 文件搜索效率/安全性稍差 |
| StrReplaceFile 精确编辑 | 字符串替换编辑 | 无（只能全文写入） | 大文件编辑风险高 |
| Ralph 自动循环 | 自动重试直到 STOP | 无 | 迭代式任务自动化缺失 |
| 多 LLM Provider | Kimi/OpenAI/Anthropic/Gemini/VertexAI | 仅 OpenAI 兼容 | 实际上够用 |
| 图片/视频输入 | ReadMediaFile | 无 | 多模态能力缺失 |
| OAuth 认证 | Kimi OAuth + keyring | 无 | 仅影响 Kimi 原生 API 用户 |

## 四、改进路线

| 优先级 | 缺失能力 | 工作量 | 补上后功力 |
|--------|----------|--------|-----------|
| P0 | LLM 错误重试（指数退避） | 小（~100 行） | 55% → 65% |
| P0 | Token 计数 + 上下文自动压缩 | 中（~300 行） | 65% → 75% |
| P1 | Streaming 接入 agent loop | 中（~200 行） | 75% → 80% |
| P1 | 工具输出截断（发给 LLM 的） | 小（~30 行） | 80% → 82% |
| P1 | 工具并行执行 | 小（~50 行，goroutine） | 82% → 85% |
| P2 | YOLO 审批系统 | 中（~200 行） | 85% → 88% |

当前评估：**~55-60%**。补完 P0+P1 可达 **~85%**，加上 P2 到 **~88%**。
