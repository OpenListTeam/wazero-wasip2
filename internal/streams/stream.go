package streams

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

	Flusher Flusher
}

// Manager 现在是 witgo.ResourceManager 的一个类型别名，专门用于管理 Stream 资源。
type Manager = witgo.ResourceManager[*Stream]

// NewManager 创建一个新的 Stream 管理器。
func NewManager() *Manager {
	return witgo.NewResourceManager[*Stream]()
}
