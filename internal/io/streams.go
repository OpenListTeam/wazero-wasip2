package io

import (
	"io"
	witgo "wazero-wasip2/wit-go"
)

// Flusher 是一个接口，封装了 Flush 方法，用于将缓冲数据写入底层 writer。
type Flusher interface {
	Flush() error
}

// Stream 结构体代表一个 WASI 流，封装了 Go 的 io 接口。
type Stream struct {
	// 可选的 Reader，用于输入流。
	Reader io.Reader
	// 可选的 Writer，用于输出流。
	Writer io.Writer
	// 可选的 Closer，用于在流被丢弃时关闭底层资源。
	Closer io.Closer
	// 可选的 Seeker, 用于优化Skip
	Seeker io.Seeker

	Flusher Flusher

	// OnSubscribe 是一个回调函数，由 Stream 的创建者提供。
	// 当 Guest 订阅此流时，该函数被调用以创建一个新的、与底层资源状态
	// 关联的 Pollable 句柄。
	OnSubscribe func() uint32 // Returns a Pollable handle

	// 使用系统级别的轮训
	Fd int
}

// Manager 现在是 witgo.ResourceManager 的一个类型别名，专门用于管理 Stream 资源。
type StreamManager = witgo.ResourceManager[*Stream]

// NewManager 创建一个新的 Stream 管理器。
func NewStreamManager() *StreamManager {
	return witgo.NewResourceManager[*Stream]()
}

func NewManager() (*StreamManager, *PollManager, *ErrorManager) {
	return NewStreamManager(), NewPollManager(), NewErrorManager()
}
