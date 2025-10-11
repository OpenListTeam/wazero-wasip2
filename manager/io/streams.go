package io

import (
	"io"

	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

// Flusher 是一个接口，封装了 Flush 方法，用于将缓冲数据写入底层 writer。
type Flusher interface {
	Flush() error
}

type CheckWriter interface {
	CheckWrite() uint64
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

	// 可选的 CheckWriter 用于检测非阻塞下允许写入的长度
	CheckWriter CheckWriter

	// OnSubscribe 是一个回调函数，由 Stream 的创建者提供。
	// 当 Guest 订阅此流时，该函数被调用以创建一个新的、与底层资源状态
	OnSubscribe func() IPollable // Returns a Pollable handle
}

// Manager 现在是 witgo.ResourceManager 的一个类型别名，专门用于管理 Stream 资源。
type StreamManager = witgo.ResourceManager[*Stream]

// NewManager 创建一个新的 Stream 管理器。
func NewStreamManager() *StreamManager {
	return witgo.NewResourceManager[*Stream](func(resource *Stream) {
		if resource.Closer != nil {
			_ = resource.Closer.Close()
		}
	})
}

func NewManager() (*StreamManager, *PollManager, *ErrorManager) {
	return NewStreamManager(), NewPollManager(), NewErrorManager()
}
