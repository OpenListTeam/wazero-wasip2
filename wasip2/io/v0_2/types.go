package v0_2

import (
	witgo "wazero-wasip2/wit-go"
)

// --- wasi:io/error types ---
type Error = uint32

// --- wasi:io/poll types ---
type Pollable = uint32

// --- wasi:io/streams types ---
type InputStream = uint32
type OutputStream = uint32

type StreamError struct {
	// 当 case 为 'last-operation-failed' 时，此字段为非 nil。
	LastOperationFailed *Error `wit:"case(0)"`
	// 当 case 为 'closed' 时，此字段为非 nil。
	// 当读取完毕返回io.EOF时也返回这个错误
	Closed *witgo.Unit `wit:"case(1)"`
}
