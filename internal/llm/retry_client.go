// Package llm provides LLM client implementations.
package llm

import (
	"context"
	"fmt"
	"time"

	"kimi-go/internal/config"
)

// Logger 定义重试日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
}

// RetryableClient 包装 LLMClient 添加重试功能
type RetryableClient struct {
	inner      *Client
	config     *RetryConfig
	logger     Logger
}

// NewRetryableClient 创建带重试功能的客户端
func NewRetryableClient(
	inner *Client,
	config *RetryConfig,
	logger Logger,
) *RetryableClient {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryableClient{
		inner:      inner,
		config:     config,
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
			if attempt > 0 && c.logger != nil {
				c.logger.Info("LLM request succeeded after %d retry(s)", attempt)
			}
			return resp, nil
		}

		lastErr = err

		// 检查是否应该重试
		if !IsRetryableError(err) {
			if c.logger != nil {
				c.logger.Debug("Non-retryable error, aborting: %v", err)
			}
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

		if c.logger != nil {
			c.logger.Info("Retrying LLM request (attempt %d/%d) after %v: %v",
				attempt+1, c.config.MaxRetries, waitTime, err)
		}

		// 等待
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.config.MaxRetries, lastErr)
}

// Chat 带重试的普通聊天调用
func (c *RetryableClient) Chat(
	ctx context.Context,
	messages []Message,
) (*ChatResponse, error) {
	return c.ChatWithTools(ctx, messages, nil)
}

// ChatStream 带重试的流式聊天调用
func (c *RetryableClient) ChatStream(
	ctx context.Context,
	messages []Message,
) (<-chan ChatResponse, <-chan error) {
	// 流式调用的重试比较复杂，暂时委托给内部客户端
	// 后续可以考虑实现断点续传的重试机制
	return c.inner.ChatStream(ctx, messages)
}

// ChatStreamWithTools 带重试的流式聊天调用（支持工具）
func (c *RetryableClient) ChatStreamWithTools(
	ctx context.Context,
	messages []Message,
	tools []ToolDef,
) (<-chan ChatResponse, <-chan error) {
	// 流式调用的重试比较复杂，暂时委托给内部客户端
	// 后续可以考虑实现断点续传的重试机制
	return c.inner.ChatStreamWithTools(ctx, messages, tools)
}

// Ensure RetryableClient implements the same interface as Client (including streaming methods)
var _ interface {
	Chat(ctx context.Context, messages []Message) (*ChatResponse, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error)
	ChatStream(ctx context.Context, messages []Message) (<-chan ChatResponse, <-chan error)
	ChatStreamWithTools(ctx context.Context, messages []Message, tools []ToolDef) (<-chan ChatResponse, <-chan error)
} = (*RetryableClient)(nil)

// RetryConfigFromProvider 从 ProviderConfig 创建 RetryConfig
func RetryConfigFromProvider(provider *config.ProviderConfig) *RetryConfig {
	if provider.Retry == nil {
		return DefaultRetryConfig()
	}

	retry := provider.Retry
	cfg := &RetryConfig{
		MaxRetries:      retry.MaxRetries,
		InitialWait:     time.Duration(retry.InitialWaitMs) * time.Millisecond,
		MaxWait:         time.Duration(retry.MaxWaitMs) * time.Millisecond,
		ExponentialBase: retry.ExponentialBase,
		Jitter:          time.Duration(retry.JitterMs) * time.Millisecond,
	}

	// 设置默认值
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = DefaultRetryConfig().MaxRetries
	}
	if cfg.InitialWait <= 0 {
		cfg.InitialWait = DefaultRetryConfig().InitialWait
	}
	if cfg.MaxWait <= 0 {
		cfg.MaxWait = DefaultRetryConfig().MaxWait
	}
	if cfg.ExponentialBase <= 1 {
		cfg.ExponentialBase = DefaultRetryConfig().ExponentialBase
	}

	return cfg
}
