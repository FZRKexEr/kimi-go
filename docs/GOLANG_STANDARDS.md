# Go 编码规范

本项目遵循的 Go 编码标准与约定。

## 参考标准

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)

## 项目约定

### 目录结构

- `cmd/` — 应用程序入口，每个子目录一个 main
- `internal/` — 私有代码，外部不可导入
- `scripts/` — 构建和验证脚本
- `docs/` — 文档

### 命名

- 包名：小写单词，不用下划线（`soul`, `llm`, `wire`）
- 接口：动词或能力名词（`Tool`, `Reader`）
- 导出函数：大驼峰（`NewClient`, `ChatWithTools`）
- 私有函数：小驼峰（`buildLLMMessages`, `extractText`）
- 测试文件：`xxx_test.go`，与被测文件同目录

### 错误处理

- 返回 `error` 而不是 panic
- 用 `fmt.Errorf("context: %w", err)` 包装错误
- 工具执行错误不终止 Agent Loop，作为结果返回给 LLM

### 测试

- 单元测试用 `net/http/httptest` mock 外部服务
- 集成测试用 `_test.go` + 环境变量守卫（`if os.Getenv("XXX") == "" { t.Skip() }`）
- 构建产物和临时文件不得污染项目目录（使用 `os.MkdirTemp`）

### 接口

```go
// Tool 接口 — 所有工具必须实现
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}
```

### 并发

- 用 `context.Context` 控制取消和超时
- 用 `sync.Mutex` 保护共享状态
- Channel 用于 goroutine 间通信（`msgCh`, `DoneCh`）

## 代码格式化

```bash
gofmt -s -w .    # 格式化
go vet ./...     # 静态分析
```
