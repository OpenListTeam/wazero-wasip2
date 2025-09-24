package tls

import (
	"crypto/tls"
	"sync"
	"sync/atomic"
	manager_io "wazero-wasip2/internal/io"
	witgo "wazero-wasip2/wit-go"
)

// ClientHandshake 代表一个客户端 TLS 握手操作。
type ClientHandshake struct {
	ServerName string
	Input      manager_io.Stream // The underlying input stream (e.g., from a TCP socket)
	Output     manager_io.Stream // The underlying output stream (e.g., from a TCP socket)
}

// ClientConnection 代表一个已建立的 TLS 连接。
type ClientConnection struct {
	Conn *tls.Conn
}

// FutureClientStreams 代表一个尚未完成的 TLS 握手，最终会产生加密流。
type FutureClientStreams struct {
	ResultChan   chan Result
	Result       atomic.Pointer[Result]
	Consumed     atomic.Bool
	Pollable     *manager_io.ChannelPollable
	PollableOnce sync.Once
}

// Result 是一个内部类型，用于在 goroutine 之间传递 TLS 握手的结果。
type Result struct {
	ConnectionHandle uint32 // 指向 ClientConnection 的句柄
	InputStream      uint32 // 加密的 input-stream
	OutputStream     uint32 // 加密的 output-stream
	Err              error  // 或一个 Go 的 error
}

// TLSManager 是所有 TLS 相关资源的总管理器。
type TLSManager struct {
	ClientHandshakes    *witgo.ResourceManager[*ClientHandshake]
	ClientConnections   *witgo.ResourceManager[*ClientConnection]
	FutureClientStreams *witgo.ResourceManager[*FutureClientStreams]
}

func NewTLSManager() *TLSManager {
	return &TLSManager{
		ClientHandshakes:    witgo.NewResourceManager[*ClientHandshake](),
		ClientConnections:   witgo.NewResourceManager[*ClientConnection](),
		FutureClientStreams: witgo.NewResourceManager[*FutureClientStreams](),
	}
}
