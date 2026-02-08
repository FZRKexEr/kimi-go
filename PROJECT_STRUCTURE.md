# Kimi-Go 项目结构

采用标准 Go 项目布局（Standard Go Project Layout）。

## 目录结构

```
kimi-go/
├── cmd/kimi/                 # CLI 入口
│   ├── main.go               # main 函数、REPL、系统提示词构建
│   └── main_test.go
│
├── internal/                 # 私有代码（不可被外部导入）
│   ├── soul/                 # 核心 Agent 逻辑
│   │   ├── soul.go           # Soul + Agent + Runtime + Agent Loop
│   │   ├── soul_test.go
│   │   ├── context.go        # 对话上下文管理、持久化
│   │   └── context_test.go
│   │
│   ├── llm/                  # LLM 客户端
│   │   ├── client.go         # Chat、ChatWithTools、Stream + 类型定义
│   │   ├── client_test.go
│   │   └── integration_test.go
│   │
│   ├── tools/                # 工具实现
│   │   ├── tool.go           # Tool 接口、ToolSet 注册表
│   │   ├── tool_test.go
│   │   ├── shell.go          # Shell 工具
│   │   ├── shell_test.go
│   │   ├── file.go           # File 工具
│   │   └── file_test.go
│   │
│   ├── wire/                 # 消息协议
│   │   └── types.go
│   │
│   ├── config/               # TOML 配置管理
│   │   ├── config.go
│   │   └── config_test.go
│   │
│   ├── session/              # 会话管理
│   │   ├── session.go
│   │   └── session_test.go
│   │
│   ├── logger/               # 日志
│   │   └── logger.go
│   │
│   └── benchmark/            # Agent 能力评测
│       ├── benchmark.go      # Runner、Report、断言检查
│       ├── benchmark_test.go
│       └── cases.go          # 评测用例定义
│
├── scripts/
│   ├── env.sh                # 环境变量配置
│   └── verify.sh             # 完整功能验证
│
├── docs/
│   ├── ARCHITECTURE.md       # 架构设计文档
│   └── GOLANG_STANDARDS.md   # Go 编码规范参考
│
├── Makefile                  # 构建脚本
├── go.mod
├── go.sum
├── README.md                 # 项目说明
├── AGENTS.md                 # AI 助手项目上下文
└── PROJECT_STRUCTURE.md      # 本文件
```

## 设计原则

- **`internal/`**: 所有核心代码在 internal 下，防止外部导入
- **单一入口**: `cmd/kimi/main.go` 是唯一的 main
- **接口驱动**: 工具通过 `Tool` 接口扩展
- **回调模式**: Soul 通过 OnMessage/OnToolCall/OnError 回调通知上层
- **测试覆盖**: 每个核心包都有 `_test.go`
