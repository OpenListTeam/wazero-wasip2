package tls

import (
	"crypto/tls"
	"sync/atomic"

	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

// ClientHandshake 代表一个客户端 TLS 握手操作。
type ClientHandshake struct {
	ServerName string
	Input      manager_io.Stream // The underlying input stream (e.g., from a TCP socket)
	Output     manager_io.Stream // The underlying output stream (e.g., from a TCP socket)
}

func (c *ClientHandshake) Close() error {
	return manager_io.NewMultiCloser(c.Input.Closer, c.Output.Closer).Close()
}

// ClientConnection 代表一个已建立的 TLS 连接。
type ClientConnection struct {
	Conn *tls.Conn
}

func (c *ClientConnection) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

// FutureClientStreams 代表一个尚未完成的 TLS 握手，最终会产生加密流。
type FutureClientStreams struct {
	Pollable *manager_io.ChannelPollable
	Result   Result
	Consumed atomic.Bool
}

func (c *FutureClientStreams) Close() error {
	if c.Result.TlsConn != nil {
		return c.Result.TlsConn.Close()
	}
	return nil
}

// Result 是一个内部类型，用于在 goroutine 之间传递 TLS 握手的结果。
type Result struct {
	TlsConn *tls.Conn
	Err     error // 或一个 Go 的 error
}

// TLSManager 是所有 TLS 相关资源的总管理器。
type TLSManager struct {
	ClientHandshakes    *witgo.ResourceManager[*ClientHandshake]
	ClientConnections   *witgo.ResourceManager[*ClientConnection]
	FutureClientStreams *witgo.ResourceManager[*FutureClientStreams]
}

func NewTLSManager() *TLSManager {
	return &TLSManager{
		ClientHandshakes: witgo.NewResourceManager[*ClientHandshake](func(resource *ClientHandshake) {
			resource.Close()
		}),
		ClientConnections: witgo.NewResourceManager[*ClientConnection](func(resource *ClientConnection) {
			resource.Close()

		}),
		FutureClientStreams: witgo.NewResourceManager[*FutureClientStreams](func(resource *FutureClientStreams) {
			resource.Close()
		}),
	}
}
