# AGENTS.md

本文件为 AI 编程助手提供项目上下文信息。

## 项目概述

kimi-go 是一个用 Go 编写的 AI 编程助手 CLI 工具。核心能力是通过 Agent Loop 将 LLM 与 Tool（Shell、File）串联，实现自动化编程任务。

## 技术栈

- Go 1.25+
- OpenAI 兼容 API（支持 tool calling）
- TOML 配置（github.com/BurntSushi/toml）
- UUID（github.com/google/uuid）

## 项目结构

```
cmd/kimi/main.go           # CLI 入口、REPL、系统提示词构建
internal/
  soul/soul.go             # 核心：Agent Loop（LLM ↔ Tool 循环）
  soul/context.go          # 对话上下文管理、持久化
  llm/client.go            # LLM 客户端（Chat、ChatWithTools、Stream）
  tools/tool.go            # Tool 接口定义、ToolSet 注册表
  tools/shell.go           # Shell 工具（命令执行）
  tools/file.go            # File 工具（读写删列）
  wire/types.go            # 消息协议类型
  config/config.go         # TOML 配置加载
  session/session.go       # 会话管理
  logger/logger.go         # 日志
  benchmark/               # Agent 能力评测框架
scripts/
  env.sh                   # 环境变量配置
  verify.sh                # 完整功能验证脚本
```

## 核心数据流

1. 用户输入 → `Soul.SendMessage()` → `msgCh`
2. `Soul.Run()` 从 `msgCh` 取消息 → `processWithLLM()`
3. 构建 system prompt + llmHistory → 调用 `LLMClient.ChatWithTools()`
4. LLM 返回 `tool_calls` → 执行 Tool → 将结果追加 → 再次调用 LLM
5. LLM 返回纯文本 → 输出响应 → 保存上下文 → 通过 `DoneCh` 通知完成

## 构建与测试

```bash
make build          # 构建到 build/kimi
make test           # 单元测试 + 覆盖率
make check          # fmt + vet + test
make verify         # 完整验证
make benchmark      # Agent 评测（需要 RUN_BENCHMARK=1 + API Key）
```

## 编码规范

- 遵循 Go 标准项目布局，核心代码在 `internal/`
- 所有核心包都有 `_test.go` 文件
- 工具实现 `tools.Tool` 接口（Name、Description、Parameters、Execute）
- LLM 相关类型在 `internal/llm/client.go`（Message、ToolDef、ToolCallInfo 等）
- 消息协议在 `internal/wire/types.go`（wire.Message、ContentPart、MessageType）
- 错误处理：工具执行错误不终止 Agent Loop，而是作为 tool result 返回给 LLM
- 测试中使用 `net/http/httptest` mock LLM 服务器
- 构建产物和临时文件不得污染项目目录

## 环境变量

| 变量 | 说明 | 必需 |
|---|---|---|
| `OPENAI_BASE_URL` | API 端点 URL | 是 |
| `OPENAI_API_KEY` | API 密钥 | 是 |
| `OPENAI_MODEL` | 模型名称 | 是 |
| `RUN_BENCHMARK` | 设为 `1` 启用 benchmark 测试 | 否 |
| `BENCHMARK_OUTPUT_DIR` | benchmark 结果输出目录 | 否 |

## 添加新工具

1. 在 `internal/tools/` 创建文件，实现 `Tool` 接口
2. 在 `cmd/kimi/main.go` 注册工具到 Runtime
3. 在 Agent 上添加工具名
4. 添加对应的 `_test.go`
5. 如有必要，在 `buildSystemPrompt()` 中更新工具描述
