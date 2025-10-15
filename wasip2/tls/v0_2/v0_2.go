package v0_2

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	manager_tls "github.com/OpenListTeam/wazero-wasip2/manager/tls"
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// streamConn 是一个包装器，它为一个 io.Reader 和 io.Writer 对实现了 net.Conn 接口，
// 以便能传递给 Go 的 crypto/tls 包。
type streamConn struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
}

func (c *streamConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *streamConn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

func (c *streamConn) Close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

// 为 net.Conn 接口的其余方法提供虚拟实现。
func (c *streamConn) LocalAddr() net.Addr                { return nil }
func (c *streamConn) RemoteAddr() net.Addr               { return nil }
func (c *streamConn) SetDeadline(t time.Time) error      { return nil }
func (c *streamConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *streamConn) SetWriteDeadline(t time.Time) error { return nil }

// --- wasi:tls/types@0.2.0-draft implementation ---

type tlsTypes struct {
	tm *manager_tls.TLSManager
	sm *manager_io.StreamManager
	em *manager_io.ErrorManager
}

func NewTypes(tm *manager_tls.TLSManager, sm *manager_io.StreamManager, em *manager_io.ErrorManager) wasip2.Implementation {
	return &tlsTypes{tm: tm, sm: sm, em: em}
}

func (i *tlsTypes) Name() string       { return "wasi:tls/types" }
func (i *tlsTypes) Versions() []string { return []string{"0.2.0-draft"} }

func (i *tlsTypes) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	exporter := witgo.NewExporter(builder)

	tm := h.TLSManager()
	sm := h.StreamManager()
	em := h.ErrorManager()

	// --- client-handshake ---
	exporter.Export("[constructor]client-handshake", func(serverName string, inputStream InputStream, outputStream OutputStream) ClientHandshake {
		inStream, inOk := sm.Pop(inputStream)
		outStream, outOk := sm.Pop(outputStream)
		if !inOk || !outOk {
			panic("invalid input or output stream for TLS handshake")
		}

		handshake := &manager_tls.ClientHandshake{
			ServerName: serverName,
			Input:      *inStream,
			Output:     *outStream,
		}
		return tm.ClientHandshakes.Add(handshake)
	})
	exporter.Export("[resource-drop]client-handshake", tm.ClientHandshakes.Remove)

	exporter.Export("[static]client-handshake.finish", func(this ClientHandshake) FutureClientStreams {
		// 根据 WIT 规范，finish 会消费掉 client-handshake 句柄。
		handshake, ok := tm.ClientHandshakes.Pop(this)
		if !ok {
			panic("invalid client-handshake handle")
		}

		future := &manager_tls.FutureClientStreams{
			Pollable: manager_io.NewPollable(nil),
		}

		futureHandle := tm.FutureClientStreams.Add(future)

		// 在后台 goroutine 中启动 TLS 握手。
		go func() {
			defer future.Pollable.SetReady()

			// 使用 streamConn 包装器来满足 net.Conn 接口。
			underlyingConn := &streamConn{
				reader: handshake.Input.Reader,
				writer: handshake.Output.Writer,
				// 假设两个流共享同一个 closer。
				closer: handshake,
			}

			tlsConn := tls.Client(underlyingConn, &tls.Config{
				ServerName: handshake.ServerName,
			})

			if err := tlsConn.Handshake(); err != nil {
				future.Result = manager_tls.Result{Err: err}
				return
			}

			future.Result = manager_tls.Result{TlsConn: tlsConn}
		}()

		return futureHandle
	})

	// --- client-connection ---
	exporter.Export("[resource-drop]client-connection", tm.ClientConnections.Remove)

	exporter.Export("[method]client-connection.close-output", func(this ClientConnection) {
		conn, ok := tm.ClientConnections.Get(this)
		if !ok {
			return
		}
		// 这会向对方发送一个 "close_notify" TLS 警报。
		conn.Conn.CloseWrite()
	})

	// --- future-client-streams ---
	exporter.Export("[resource-drop]future-client-streams", tm.FutureClientStreams.Remove)
	exporter.Export("[method]future-client-streams.subscribe", func(this FutureClientStreams) Pollable {
		future, ok := tm.FutureClientStreams.Get(this)
		if !ok {
			return h.PollManager().Add(manager_io.ReadyPollable)
		}

		// future.Pollable 是没有close副作用的，可以使用同一个
		return h.PollManager().Add(future.Pollable)
	})

	exporter.Export("[method]future-client-streams.get", func(ctx context.Context, this FutureClientStreams) witgo.Option[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]] {
		future, ok := tm.FutureClientStreams.Pop(this)
		if !ok {
			return witgo.None[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]]()
		}

		select {
		case <-future.Pollable.Channel():
		case <-ctx.Done():
			return witgo.None[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]]()
		}

		if !future.Consumed.CompareAndSwap(false, true) {
			return witgo.Some(witgo.Err[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit](witgo.Unit{}))
		}

		if future.Result.Err != nil {
			errHandle := em.Add(future.Result.Err)
			return witgo.Some(witgo.Ok[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit](witgo.Err[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError](errHandle)))
		}

		tlsConn := future.Result.TlsConn
		// 为加密连接创建新的异步流。
		inStreamEncrypted := manager_io.NewAsyncStreamForReader(tlsConn)
		outStreamEncrypted := manager_io.NewAsyncStreamForWriter(tlsConn)

		inStreamHandle := sm.Add(inStreamEncrypted)
		outStreamHandle := sm.Add(outStreamEncrypted)

		// 创建 client-connection 资源。
		conn := &manager_tls.ClientConnection{Conn: tlsConn}
		connHandle := tm.ClientConnections.Add(conn)

		tuple := witgo.Tuple3[ClientConnection, InputStream, OutputStream]{
			F0: connHandle,
			F1: inStreamHandle,
			F2: outStreamHandle,
		}
		return witgo.Some(witgo.Ok[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit](witgo.Ok[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError](tuple)))
	})

	return nil
}
