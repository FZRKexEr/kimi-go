# Kimi-Go

基于 OpenAI 兼容 API 的 AI 编程助手 CLI，Go 实现。

## 功能

- 交互式多轮对话
- Tool Calling（Shell 命令执行 + 文件操作）
- Agent Loop（LLM → Tool → LLM 自动循环）
- 会话持久化与恢复
- 动态系统提示词（自动注入项目上下文）
- 支持任意 OpenAI 兼容 API

## 快速开始

### 1. 配置环境变量

```bash
export OPENAI_BASE_URL="https://your-api-endpoint"
export OPENAI_API_KEY="your-api-key"
export OPENAI_MODEL="your-model-name"
```

或使用项目提供的脚本：

```bash
source scripts/env.sh
```

### 2. 构建并运行

```bash
make build
make run
```

或直接开发模式运行（不产生二进制文件）：

```bash
make dev
```

### 3. 使用

```
> 你好
Assistant: 你好！有什么可以帮你的吗？

> 列出当前目录的文件
[Tool Call] Calling tool: shell({"command":"ls -la"})
[Tool Result] ...
Assistant: 当前目录包含以下文件...

> exit
```

## 构建

```bash
make build        # 构建到 build/ 目录
make install      # 安装到 /usr/local/bin（需要 sudo）
make clean        # 清理构建产物
```

## 测试

```bash
make test          # 运行所有单元测试（含覆盖率和 race detector）
make test-short    # 快速测试（不含 race detector）
make check         # fmt + vet + test
make verify        # 完整验证（编译 + 单测 + 功能测试 + 代码质量）
make benchmark     # Agent 能力评测（需要 API Key）
```

## 项目结构

```
kimi-go/
├── cmd/kimi/              # CLI 入口
├── internal/
│   ├── soul/              # 核心 Agent 逻辑（Soul + Agent + Runtime）
│   ├── llm/               # LLM 客户端（OpenAI 兼容，支持 Tool Calling）
│   ├── tools/             # 工具实现（Shell、File）
│   ├── wire/              # 消息协议类型
│   ├── config/            # 配置管理（TOML）
│   ├── session/           # 会话管理
│   ├── logger/            # 日志
│   └── benchmark/         # Agent 能力评测框架
├── scripts/               # 辅助脚本
├── docs/                  # 设计文档
└── Makefile
```

## 架构

```
用户输入 → CLI → Soul (Agent Loop)
                    ├── 构建 System Prompt + 历史消息
                    ├── 调用 LLM Client (ChatWithTools)
                    ├── 如果返回 tool_calls:
                    │   ├── 执行 Tool（Shell/File）
                    │   ├── 将结果追加到消息列表
                    │   └── 继续循环
                    └── 如果返回文本: 输出响应，结束
```

## 配置

支持 TOML 配置文件（`~/.config/kimi/config.toml`）或环境变量。

环境变量优先级高于配置文件：

| 环境变量 | 说明 |
|---|---|
| `OPENAI_BASE_URL` | API 端点 |
| `OPENAI_API_KEY` | API 密钥 |
| `OPENAI_MODEL` | 模型名称 |

## 命令行参数

```
-config     配置文件路径
-work-dir   工作目录（默认当前目录）
-session    恢复指定会话
-yolo       自动批准所有操作
-version    显示版本
```
