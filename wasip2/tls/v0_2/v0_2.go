package v0_2

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	manager_io "wazero-wasip2/internal/io"
	manager_tls "wazero-wasip2/internal/tls"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"

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
		inStream, inOk := sm.Get(inputStream)
		outStream, outOk := sm.Get(outputStream)
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
		handshake, ok := tm.ClientHandshakes.Get(this)
		if !ok {
			panic("invalid client-handshake handle")
		}
		// 根据 WIT 规范，finish 会消费掉 client-handshake 句柄。
		tm.ClientHandshakes.Remove(this)

		future := &manager_tls.FutureClientStreams{
			ResultChan: make(chan manager_tls.Result, 1),
		}
		futureHandle := tm.FutureClientStreams.Add(future)

		// 在后台 goroutine 中启动 TLS 握手。
		go func() {
			// 使用 streamConn 包装器来满足 net.Conn 接口。
			underlyingConn := &streamConn{
				reader: handshake.Input.Reader,
				writer: handshake.Output.Writer,
				// 假设两个流共享同一个 closer。
				closer: handshake.Input.Closer,
			}

			tlsConn := tls.Client(underlyingConn, &tls.Config{
				ServerName: handshake.ServerName,
			})

			if err := tlsConn.Handshake(); err != nil {
				future.ResultChan <- manager_tls.Result{Err: err}
				return
			}

			// 为加密连接创建新的异步流。
			inStreamEncrypted := manager_io.NewAsyncStreamForReader(tlsConn)
			outStreamEncrypted := manager_io.NewAsyncStreamForWriter(tlsConn)

			inStreamHandle := sm.Add(inStreamEncrypted)
			outStreamHandle := sm.Add(outStreamEncrypted)

			// 创建 client-connection 资源。
			conn := &manager_tls.ClientConnection{Conn: tlsConn}
			connHandle := tm.ClientConnections.Add(conn)

			// 发送成功结果。
			future.ResultChan <- manager_tls.Result{
				ConnectionHandle: connHandle,
				InputStream:      inStreamHandle,
				OutputStream:     outStreamHandle,
			}
		}()

		return futureHandle
	})

	// --- client-connection ---
	exporter.Export("[resource-drop]client-connection", func(handle ClientConnection) {
		conn, ok := tm.ClientConnections.Get(handle)
		if ok {
			conn.Conn.Close()
		}
		tm.ClientConnections.Remove(handle)
	})

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

		future.PollableOnce.Do(func() {
			ctx, cancel := context.WithCancel(context.Background())
			future.Pollable = manager_io.NewPollable(cancel)
			go func() {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-future.ResultChan:
					if ok {
						future.Result.Store(&res)
					}
					future.Pollable.SetReady()
				}
			}()
		})
		return h.PollManager().Add(future.Pollable)
	})

	exporter.Export("[method]future-client-streams.get", func(this FutureClientStreams) witgo.Option[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]] {
		future, ok := tm.FutureClientStreams.Get(this)
		if !ok {
			return witgo.None[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]]()
		}

		res := future.Result.Load()
		if res == nil {
			return witgo.None[witgo.Result[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit]]()
		}

		if !future.Consumed.CompareAndSwap(false, true) {
			return witgo.Some(witgo.Err[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit](witgo.Unit{}))
		}

		var innerResult witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError]
		if res.Err != nil {
			errHandle := em.Add(res.Err)
			innerResult = witgo.Err[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError](errHandle)
		} else {
			tuple := witgo.Tuple3[ClientConnection, InputStream, OutputStream]{
				F0: res.ConnectionHandle,
				F1: res.InputStream,
				F2: res.OutputStream,
			}
			innerResult = witgo.Ok[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError](tuple)
		}

		return witgo.Some(witgo.Ok[witgo.Result[witgo.Tuple3[ClientConnection, InputStream, OutputStream], WasiError], witgo.Unit](innerResult))
	})

	return nil
}
