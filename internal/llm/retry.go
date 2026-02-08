// Package llm provides LLM client implementations.
package llm

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig 定义重试策略配置
type RetryConfig struct {
	MaxRetries       int           // 最大重试次数，默认 3
	InitialWait      time.Duration // 初始等待时间，默认 300ms
	MaxWait          time.Duration // 最大等待时间，默认 5s
	ExponentialBase  float64       // 指数基数，默认 2
	Jitter           time.Duration // 抖动范围，默认 500ms
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:       3,
		InitialWait:      300 * time.Millisecond,
		MaxWait:          5 * time.Second,
		ExponentialBase:  2.0,
		Jitter:           500 * time.Millisecond,
	}
}

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

	// 确保等待时间不为负
	if wait < 0 {
		wait = 0
	}

	return wait
}

// RetryableFunc 表示可重试的函数类型
type RetryableFunc func(ctx context.Context) error

// ExecuteWithRetry 执行带重试的函数
func ExecuteWithRetry(
	ctx context.Context,
	config *RetryConfig,
	fn RetryableFunc,
	isRetryable func(error) bool,
	onRetry func(attempt int, err error, waitTime time.Duration),
) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行函数
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否应该重试
		if isRetryable != nil && !isRetryable(err) {
			return err
		}

		// 如果是最后一次尝试，不再等待
		if attempt >= config.MaxRetries {
			break
		}

		// 计算等待时间
		waitTime := ExponentialBackoff(
			attempt,
			config.InitialWait,
			config.MaxWait,
			config.ExponentialBase,
			config.Jitter,
		)

		// 回调通知
		if onRetry != nil {
			onRetry(attempt+1, err, waitTime)
		}

		// 等待
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}
