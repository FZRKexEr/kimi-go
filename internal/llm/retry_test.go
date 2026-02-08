// Package llm provides LLM client implementations.
package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialWait != 300*time.Millisecond {
		t.Errorf("expected InitialWait=300ms, got %v", cfg.InitialWait)
	}
	if cfg.MaxWait != 5*time.Second {
		t.Errorf("expected MaxWait=5s, got %v", cfg.MaxWait)
	}
	if cfg.ExponentialBase != 2.0 {
		t.Errorf("expected ExponentialBase=2.0, got %f", cfg.ExponentialBase)
	}
	if cfg.Jitter != 500*time.Millisecond {
		t.Errorf("expected Jitter=500ms, got %v", cfg.Jitter)
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		name          string
		attempt       int
		initialWait   time.Duration
		maxWait       time.Duration
		base          float64
		jitter        time.Duration
		expectedMin   time.Duration
		expectedMax   time.Duration
	}{
		{
			name:        "first attempt with defaults",
			attempt:     0,
			initialWait: 300 * time.Millisecond,
			maxWait:     5 * time.Second,
			base:        2.0,
			jitter:      500 * time.Millisecond,
			expectedMin: 0,
			expectedMax: 300*time.Millisecond + 500*time.Millisecond,
		},
		{
			name:        "second attempt with defaults",
			attempt:     1,
			initialWait: 300 * time.Millisecond,
			maxWait:     5 * time.Second,
			base:        2.0,
			jitter:      500 * time.Millisecond,
			expectedMin: 300*2*time.Millisecond - 500*time.Millisecond,
			expectedMax: 300*2*time.Millisecond + 500*time.Millisecond,
		},
		{
			name:        "max wait cap",
			attempt:     10,
			initialWait: 300 * time.Millisecond,
			maxWait:     1 * time.Second,
			base:        2.0,
			jitter:      100 * time.Millisecond,
			expectedMin: 1*time.Second - 100*time.Millisecond,
			expectedMax: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for jitter
			for i := 0; i < 10; i++ {
				result := ExponentialBackoff(tt.attempt, tt.initialWait, tt.maxWait, tt.base, tt.jitter)

				if result < tt.expectedMin {
					t.Errorf("ExponentialBackoff() = %v, below minimum %v", result, tt.expectedMin)
				}
				if result > tt.expectedMax {
					t.Errorf("ExponentialBackoff() = %v, above maximum %v", result, tt.expectedMax)
				}
			}
		})
	}
}

func TestExecuteWithRetry_Success(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialWait:       10 * time.Millisecond,
		MaxWait:           100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return nil
	}

	err := ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, nil)
	if err != nil {
		t.Errorf("ExecuteWithRetry() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestExecuteWithRetry_RetryThenSuccess(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialWait:       10 * time.Millisecond,
		MaxWait:           100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return &APIError{StatusCode: 503, Message: "service unavailable"}
		}
		return nil
	}

	retryLog := []struct {
		attempt  int
		waitTime time.Duration
	}{}
	onRetry := func(attempt int, err error, waitTime time.Duration) {
		retryLog = append(retryLog, struct {
			attempt  int
			waitTime time.Duration
		}{attempt, waitTime})
	}

	err := ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, onRetry)
	if err != nil {
		t.Errorf("ExecuteWithRetry() error = %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if len(retryLog) != 2 {
		t.Errorf("expected 2 retry logs, got %d", len(retryLog))
	}
}

func TestExecuteWithRetry_MaxRetriesExceeded(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      2,
		InitialWait:       10 * time.Millisecond,
		MaxWait:           100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return &APIError{StatusCode: 503, Message: "service unavailable"}
	}

	err := ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, nil)
	if err == nil {
		t.Error("ExecuteWithRetry() expected error, got nil")
	}
	if callCount != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if err != nil {
		// Check that error contains expected message
		expected := "max retries (2) exceeded"
		if len(err.Error()) >= len(expected) && err.Error()[:len(expected)] != expected {
			t.Errorf("expected error to start with %q, got %q", expected, err.Error())
		}
	}
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialWait:       10 * time.Millisecond,
		MaxWait:           100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return &APIError{StatusCode: 400, Message: "bad request"} // Non-retryable
	}

	err := ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, nil)
	if err == nil {
		t.Error("ExecuteWithRetry() expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call for non-retryable error, got %d", callCount)
	}
}

func TestExecuteWithRetry_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      3,
		InitialWait:       100 * time.Millisecond,
		MaxWait:           1 * time.Second,
		ExponentialBase: 2.0,
		Jitter:          10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		if callCount >= 2 {
			cancel() // Cancel context during retry
		}
		return &APIError{StatusCode: 503, Message: "service unavailable"}
	}

	err := ExecuteWithRetry(ctx, config, fn, IsRetryableError, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// TestExponentialBackoff_NoOverflow 测试指数退避在大量重试时不会溢出
func TestExponentialBackoff_NoOverflow(t *testing.T) {
	// 测试大量重试次数时不会溢出
	result := ExponentialBackoff(
		100, // 大量重试次数
		1*time.Millisecond,
		10*time.Second,
		2.0,
		1*time.Millisecond,
	)

	if result < 0 {
		t.Errorf("ExponentialBackoff() returned negative value: %v", result)
	}

	if result > 10*time.Second {
		t.Errorf("ExponentialBackoff() exceeded max: got %v, max %v", result, 10*time.Second)
	}
}

// TestExponentialBackoff_ZeroJitter 测试零抖动
func TestExponentialBackoff_ZeroJitter(t *testing.T) {
	// 多次运行，结果应该相同（因为没有抖动）
	var firstResult time.Duration
	for i := 0; i < 5; i++ {
		result := ExponentialBackoff(2, 100*time.Millisecond, 10*time.Second, 2.0, 0)
		if i == 0 {
			firstResult = result
		} else if result != firstResult {
			t.Errorf("ExponentialBackoff() with zero jitter should return consistent results: got %v and %v", firstResult, result)
		}
	}
}

// TestExecuteWithRetry_ZeroMaxRetries 测试零最大重试次数
func TestExecuteWithRetry_ZeroMaxRetries(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      0, // 不重试
		InitialWait:     10 * time.Millisecond,
		MaxWait:         100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	callCount := 0
	fn := func(ctx context.Context) error {
		callCount++
		return &APIError{StatusCode: 503, Message: "service unavailable"}
	}

	err := ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, nil)
	if err == nil {
		t.Error("ExecuteWithRetry() expected error, got nil")
	}
	if callCount != 1 { // 只调用一次，不重试
		t.Errorf("expected 1 call with zero max retries, got %d", callCount)
	}
}

// TestExecuteWithRetry_CallbackInvocation 测试回调函数被正确调用
func TestExecuteWithRetry_CallbackInvocation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      2,
		InitialWait:     10 * time.Millisecond,
		MaxWait:         100 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          5 * time.Millisecond,
	}

	fn := func(ctx context.Context) error {
		return &APIError{StatusCode: 503, Message: "service unavailable"}
	}

	callbackInvocations := 0
	var lastAttempt int
	var lastWaitTime time.Duration
	onRetry := func(attempt int, err error, waitTime time.Duration) {
		callbackInvocations++
		lastAttempt = attempt
		lastWaitTime = waitTime
	}

	ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, onRetry)

	if callbackInvocations != 2 {
		t.Errorf("expected callback to be invoked 2 times, got %d", callbackInvocations)
	}
	if lastAttempt != 2 {
		t.Errorf("expected last attempt to be 2, got %d", lastAttempt)
	}
	if lastWaitTime <= 0 {
		t.Errorf("expected last wait time to be positive, got %v", lastWaitTime)
	}
}

// TestIsRetryableError 测试错误是否可重试的判断
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "retryable API error - 429",
			err:      &APIError{StatusCode: 429, Message: "too many requests"},
			expected: true,
		},
		{
			name:     "retryable API error - 500",
			err:      &APIError{StatusCode: 500, Message: "internal error"},
			expected: true,
		},
		{
			name:     "retryable API error - 502",
			err:      &APIError{StatusCode: 502, Message: "bad gateway"},
			expected: true,
		},
		{
			name:     "retryable API error - 503",
			err:      &APIError{StatusCode: 503, Message: "service unavailable"},
			expected: true,
		},
		{
			name:     "retryable API error - 504",
			err:      &APIError{StatusCode: 504, Message: "gateway timeout"},
			expected: true,
		},
		{
			name:     "non-retryable API error - 400",
			err:      &APIError{StatusCode: 400, Message: "bad request"},
			expected: false,
		},
		{
			name:     "non-retryable API error - 401",
			err:      &APIError{StatusCode: 401, Message: "unauthorized"},
			expected: false,
		},
		{
			name:     "non-retryable API error - 403",
			err:      &APIError{StatusCode: 403, Message: "forbidden"},
			expected: false,
		},
		{
			name:     "non-retryable API error - 404",
			err:      &APIError{StatusCode: 404, Message: "not found"},
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestAPIError_IsRetryable 测试 APIError 的 IsRetryable 方法
func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{200, false},
		{0, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			err := &APIError{StatusCode: tt.statusCode, Message: "test"}
			if got := err.IsRetryable(); got != tt.expected {
				t.Errorf("APIError.IsRetryable() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

// TestAPIError_Error 测试 APIError 的 Error 方法
func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		expected   string
	}{
		{
			name:       "with status code",
			statusCode: 500,
			message:    "internal error",
			expected:   "API error (status 500): internal error",
		},
		{
			name:       "without status code",
			statusCode: 0,
			message:    "connection failed",
			expected:   "API error: connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{StatusCode: tt.statusCode, Message: tt.message}
			if got := err.Error(); got != tt.expected {
				t.Errorf("APIError.Error() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

// BenchmarkExponentialBackoff 对指数退避计算进行基准测试
func BenchmarkExponentialBackoff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ExponentialBackoff(
			5,
			100*time.Millisecond,
			5*time.Second,
			2.0,
			100*time.Millisecond,
		)
	}
}

// TestExponentialBackoff_Math 验证指数退避的数学计算
func TestExponentialBackoff_Math(t *testing.T) {
	// 测试指数退避公式: wait = initial * 2^attempt
	tests := []struct {
		attempt     int
		initialWait time.Duration
		expected    time.Duration
	}{
		{0, 100 * time.Millisecond, 100 * time.Millisecond},
		{1, 100 * time.Millisecond, 200 * time.Millisecond},
		{2, 100 * time.Millisecond, 400 * time.Millisecond},
		{3, 100 * time.Millisecond, 800 * time.Millisecond},
		{4, 100 * time.Millisecond, 1600 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			// 使用零抖动来验证数学公式
			result := ExponentialBackoff(tt.attempt, tt.initialWait, 10*time.Second, 2.0, 0)

			// 允许小误差（由于浮点数计算）
			diff := math.Abs(float64(result - tt.expected))
			if diff > float64(time.Millisecond) {
				t.Errorf("ExponentialBackoff() = %v, expected ~%v", result, tt.expected)
			}
		})
	}
}

// TestExecuteWithRetry_Concurrent 测试并发安全性
func TestExecuteWithRetry_Concurrent(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:      2,
		InitialWait:       5 * time.Millisecond,
		MaxWait:           50 * time.Millisecond,
		ExponentialBase: 2.0,
		Jitter:          2 * time.Millisecond,
	}

	var callCount int
	fn := func(ctx context.Context) error {
		callCount++
		return nil
	}

	// 并发执行多个重试操作
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- ExecuteWithRetry(context.Background(), config, fn, IsRetryableError, nil)
		}()
	}

	// 等待所有完成
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent ExecuteWithRetry() error = %v", err)
		}
	}

	if callCount != 10 {
		t.Errorf("expected 10 calls, got %d", callCount)
	}
}
