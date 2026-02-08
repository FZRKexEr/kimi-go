// Package llm provides LLM client implementations.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// APIError 表示 API 调用错误
type APIError struct {
	StatusCode int
	Message    string
	RawBody    string
}

// Error 返回错误信息
func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error: %s", e.Message)
}

// IsRetryable 判断错误是否可重试
func (e *APIError) IsRetryable() bool {
	switch e.StatusCode {
	case http.StatusTooManyRequests, // 429
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

// Error 返回错误信息
func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error (%s): %v", e.Op, e.Err)
}

// IsRetryable 判断错误是否可重试
func (e *NetworkError) IsRetryable() bool {
	// 所有网络错误都可以重试
	return true
}

// TimeoutError 表示请求超时错误
type TimeoutError struct {
	Operation string
	Duration  time.Duration
}

// Error 返回错误信息
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout error (%s after %v)", e.Operation, e.Duration)
}

// IsRetryable 判断错误是否可重试
func (e *TimeoutError) IsRetryable() bool {
	// 超时错误可以重试
	return true
}

// EmptyResponseError 表示空响应错误
type EmptyResponseError struct {
	Message string
}

// Error 返回错误信息
func (e *EmptyResponseError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("empty response: %s", e.Message)
	}
	return "empty response"
}

// IsRetryable 判断错误是否可重试
func (e *EmptyResponseError) IsRetryable() bool {
	// 空响应可能是网络问题，可以重试
	return true
}

// RetryableError 接口，用于判断错误是否可重试
type RetryableError interface {
	error
	IsRetryable() bool
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// 检查是否实现了 RetryableError 接口
	if retryable, ok := err.(RetryableError); ok {
		return retryable.IsRetryable()
	}
	
	// 检查是否是网络错误
	if isNetworkError(err) {
		return true
	}
	
	// 检查是否是超时错误
	if isTimeoutError(err) {
		return true
	}
	
	return false
}

// isNetworkError 检查是否是网络错误
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	
	// 检查 net.Error 接口
	if netErr, ok := err.(net.Error); ok {
		// 可重试的网络错误
		return netErr.Temporary() || netErr.Timeout()
	}
	
	// 检查特定错误类型
	var errno syscall.Errno
	if errors.As(err, &errno) {
		// 常见的可重试系统错误
		switch errno {
		case syscall.ECONNREFUSED,
			syscall.ECONNRESET,
			syscall.ETIMEDOUT,
			syscall.EPIPE,
			syscall.ENETUNREACH,
			syscall.EHOSTUNREACH:
			return true
		}
	}
	
	return false
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	
	// 检查 net.Error 接口
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	
	// 检查 context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	
	return false
}


