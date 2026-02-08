# 技术文档: LLM 错误重试机制

## 设计概述

采用装饰器模式实现重试逻辑，将重试逻辑与核心业务逻辑分离。使用 Go 标准库 `context` 和 `time` 实现指数退避。

## 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      Client.ChatWithTools                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   RetryableClient (Wrapper)                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  for attempt := 0; attempt <= maxRetries; attempt++ │   │
│  │      result, err = innerClient.Call()               │   │
│  │      if !isRetryable(err) { break }                  │   │
│  │      sleep(exponentialBackoff(attempt))             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Base Client (HTTP)                       │
└─────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. RetryConfig 配置结构

```go
// RetryConfig 定义重试策略配置
type RetryConfig struct {
    MaxRetries      int           // 最大重试次数，默认 3
    InitialWait     time.Duration // 初始等待时间，默认 300ms
    MaxWait         time.Duration // 最大等待时间，默认 5s
    ExponentialBase float64       // 指数基数，默认 2
    Jitter          time.Duration // 抖动范围，默认 500ms
}
```

### 2. ErrorClassifier 错误分类器

```go
// RetryableError 标识可重试的错误
type RetryableError interface {
    error
    IsRetryable() bool
}

// ErrorClassifier 判断错误是否应该重试
type ErrorClassifier interface {
    IsRetryable(err error) bool
}

// DefaultErrorClassifier 默认错误分类器
type DefaultErrorClassifier struct {
    RetryableStatusCodes []int  // 可重试的 HTTP 状态码
    RetryableErrors      []string // 可重试的错误关键词
}
```

**可重试的错误类型：**

| 错误类型 | HTTP 状态码 | 说明 |
|---------|------------|------|
| RateLimit | 429 | 请求过于频繁，需要等待后重试 |
| InternalServerError | 500 | 服务器内部错误，可能是暂时的 |
| BadGateway | 502 | 网关错误，通常是网络暂时问题 |
| ServiceUnavailable | 503 | 服务不可用，需要等待后重试 |
| GatewayTimeout | 504 | 网关超时，可以重试 |
| ConnectionError | - | 连接失败（网络问题） |
| TimeoutError | - | 请求超时 |
| EmptyResponse | - | 空响应 |

**不可重试的错误类型：**

| 错误类型 | HTTP 状态码 | 说明 |
|---------|------------|------|
| BadRequest | 400 | 请求参数错误，重试不会解决 |
| Unauthorized | 401 | 认证失败，需要检查 API Key |
| Forbidden | 403 | 权限不足，重试不会解决 |
| NotFound | 404 | 资源不存在 |

### 3. ExponentialBackoff 指数退避算法

```go
// ExponentialBackoff 计算下一次重试的等待时间
func ExponentialBackoff(
    attempt int,
    initialWait time.Duration,
    maxWait time.Duration,
    base float64,
    jitter time.Duration,
) time.Duration {
    // 计算指数退避时间
    backoff := float64(initialWait) * math.Pow(base, float64(attempt))
    
    // 添加抖动，避免惊群效应
    if jitter > 0 {
        jitterValue := time.Duration(rand.Int63n(int64(2*jitter))) - jitter
        backoff += float64(jitterValue)
    }
    
    // 限制最大等待时间
    wait := time.Duration(backoff)
    if wait > maxWait {
        wait = maxWait
    }
    
    return wait
}
```

**退避时间示例**（initial=300ms, max=5s, base=2, jitter=500ms）：

| 重试次数 | 指数退避 | +抖动 | 最终等待时间 |
|---------|---------|-------|------------|
| 0 (首次) | 300ms | ±500ms | 0ms（立即执行） |
| 1 | 600ms | ±500ms | 100ms - 1100ms |
| 2 | 1200ms | ±500ms | 700ms - 1700ms |
| 3 | 2400ms | ±500ms | 1900ms - 2900ms |
| 4 | 4800ms | ±500ms | 4300ms - 5300ms (限制为 5s) |

### 4. RetryableClient 重试包装器

```go
// RetryableClient 包装 LLMClient 添加重试功能
type RetryableClient struct {
    inner     *Client
    config    *RetryConfig
    classifier ErrorClassifier
    logger    Logger
}

// NewRetryableClient 创建带重试功能的客户端
func NewRetryableClient(
    inner *Client,
    config *RetryConfig,
    classifier ErrorClassifier,
    logger Logger,
) *RetryableClient {
    return &RetryableClient{
        inner:      inner,
        config:     config,
        classifier: classifier,
        logger:     logger,
    }
}

// ChatWithTools 带重试的聊天调用
func (c *RetryableClient) ChatWithTools(
    ctx context.Context,
    messages []Message,
    tools []ToolDef,
) (*ChatResponse, error) {
    var lastErr error
    
    for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
        // 检查上下文是否已取消
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        
        // 执行调用
        resp, err := c.inner.ChatWithTools(ctx, messages, tools)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // 检查是否应该重试
        if !c.classifier.IsRetryable(err) {
            c.logger.Debug("Non-retryable error, aborting: %v", err)
            return nil, err
        }
        
        // 如果是最后一次尝试，不再等待
        if attempt >= c.config.MaxRetries {
            break
        }
        
        // 计算等待时间
        waitTime := ExponentialBackoff(
            attempt,
            c.config.InitialWait,
            c.config.MaxWait,
            c.config.ExponentialBase,
            c.config.Jitter,
        )
        
        c.logger.Info("Retrying LLM request (attempt %d/%d) after %v: %v",
            attempt+1, c.config.MaxRetries, waitTime, err)
        
        // 等待
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(waitTime):
        }
    }
    
    return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.config.MaxRetries, lastErr)
}
```

## 配置集成

### 配置结构更新

```go
// OpenAIConfig 定义 OpenAI 兼容 API 的配置
type OpenAIConfig struct {
    BaseURL     string        `toml:"base_url"`
    APIKey      string        `toml:"api_key"`
    Model       string        `toml:"model"`
    Timeout     time.Duration `toml:"timeout"`
    
    // 重试配置
    MaxRetries      int           `toml:"max_retries"`
    RetryInitialWait time.Duration `toml:"retry_initial_wait"`
    RetryMaxWait    time.Duration `toml:"retry_max_wait"`
}
```

### 配置示例

```toml
[llm]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-4"
timeout = 120

# 重试配置
max_retries = 3           # 最大重试次数
retry_initial_wait = 300   # 初始等待时间（毫秒）
retry_max_wait = 5000     # 最大等待时间（毫秒）
```

## 错误类型定义

```go
package llm

import (
    "fmt"
    "net/http"
)

// APIError 表示 API 调用错误
type APIError struct {
    StatusCode int
    Message    string
    RawBody    string
}

func (e *APIError) Error() string {
    if e.StatusCode > 0 {
        return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
    }
    return fmt.Sprintf("API error: %s", e.Message)
}

// IsRetryable 判断错误是否可重试
func (e *APIError) IsRetryable() bool {
    switch e.StatusCode {
    case http.StatusTooManyRequests,     // 429
        http.StatusInternalServerError,    // 500
        http.StatusBadGateway,            // 502
        http.StatusServiceUnavailable,    // 503
        http.StatusGatewayTimeout:        // 504
        return true
    default:
        return false
    }
}

// NetworkError 表示网络连接错误
type NetworkError struct {
    Op  string
    Err error
}

func (e *NetworkError) Error() string {
    return fmt.Sprintf("network error (%s): %v", e.Op, e.Err)
}

func (e *NetworkError) IsRetryable() bool {
    // 所有网络错误都可以重试
    return true
}

// TimeoutError 表示请求超时错误
type TimeoutError struct {
    Operation string
    Duration  time.Duration
}

func (e *TimeoutError) Error() string {
    return fmt.Sprintf("timeout error (%s after %v)", e.Operation, e.Duration)
}

func (e *TimeoutError) IsRetryable() bool {
    // 超时错误可以重试
    return true
}
```

## 实现步骤

1. **创建 `internal/llm/retry.go`**
   - 实现指数退避算法
   - 实现错误分类器
   - 实现重试包装器

2. **创建 `internal/llm/errors.go`**
   - 定义各种错误类型
   - 实现 IsRetryable 接口

3. **修改 `internal/llm/client.go`**
   - 将 HTTP 错误转换为特定错误类型
   - 集成重试机制

4. **修改 `internal/config/openai_config.go`**
   - 添加重试相关配置项

5. **修改 `cmd/kimi/main.go`**
   - 根据配置创建带重试的客户端

6. **创建单元测试**
   - 测试指数退避计算
   - 测试错误分类
   - 测试重试逻辑

7. **创建集成测试**
   - 模拟各种错误场景
   - 验证重试行为

## 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|-----|-------|------|---------|
| 重试导致 API 限流加剧 | 中 | 高 | 添加指数退避和抖动；对 429 使用更长退避 |
| 重试延迟导致用户体验差 | 中 | 中 | 记录重试日志让用户了解进度；设置合理的最大等待时间 |
| 重试导致重复操作 | 低 | 高 | LLM 调用是幂等的（只读取，不修改） |
| 配置不兼容 | 低 | 中 | 提供默认值，保持向后兼容 |

## 后续优化

1. **断路器模式**：当连续失败达到一定阈值时，暂时停止重试，避免雪崩效应
2. **自适应重试**：根据 API 返回的 Retry-After 头动态调整重试时间
3. **可观测性增强**：添加重试次数指标，用于监控和告警
