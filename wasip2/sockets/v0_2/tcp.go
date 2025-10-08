package v0_2

import (
	"context"
	"net"

	manager_io "github.com/foxxorcat/wazero-wasip2/manager/io"
	"github.com/foxxorcat/wazero-wasip2/manager/sockets"
	"github.com/foxxorcat/wazero-wasip2/wasip2"
	wasip2_io "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type tcpImpl struct {
	host *wasip2.Host
}

func newTCPImpl(h *wasip2.Host) *tcpImpl {
	return &tcpImpl{host: h}
}

func (i *tcpImpl) StartConnect(ctx context.Context, this TCPSocket, network Network, remoteAddress IPSocketAddress) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	// 只能从未绑定或已绑定的状态开始连接
	if sock.State != sockets.TCPStateUnbound && sock.State != sockets.TCPStateBound {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	addr, err := fromIPSocketAddressToTCPAddr(remoteAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	sock.State = sockets.TCPStateConnecting
	sock.ConnectResult = make(chan sockets.ConnectResult, 1) // 创建带缓冲的 channel

	// 在后台 goroutine 中执行阻塞的 Dial 操作
	go func() {
		// net.DialTCP 会处理隐式绑定（如果套接字未绑定）
		conn, dialErr := net.DialTCP("tcp", nil, addr)
		sock.ConnectResult <- sockets.ConnectResult{Conn: conn, Err: dialErr}
	}()

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) FinishConnect(ctx context.Context, this TCPSocket) witgo.Result[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeInvalidArgument)
	}

	if sock.State != sockets.TCPStateConnecting {
		return witgo.Err[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeNotInProgress)
	}

	// 非阻塞地检查连接结果
	select {
	case result := <-sock.ConnectResult:
		if result.Err != nil {
			sock.State = sockets.TCPStateClosed
			return witgo.Err[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](mapOsError(result.Err))
		}

		// 连接成功
		sock.Conn = result.Conn
		sock.State = sockets.TCPStateConnected

		// 为连接创建输入输出流
		inStream := manager_io.NewAsyncStreamForReader(sock.Conn, manager_io.DontCloseReader())
		inStreamHandle := i.host.StreamManager().Add(inStream)
		outStream := manager_io.NewAsyncStreamForWriter(sock.Conn, manager_io.DontCloseWriter())
		outStreamHandle := i.host.StreamManager().Add(outStream)

		return witgo.Ok[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream]{
			F0: inStreamHandle,
			F1: outStreamHandle,
		})

	default:
		// 连接仍在进行中
		return witgo.Err[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeWouldBlock)
	}
}

func (i *tcpImpl) StartListen(ctx context.Context, this TCPSocket) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateBound {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	// 在 Go 中，Listen 已经在 StartBind 中完成了。这里我们只改变状态。
	sock.State = sockets.TCPStateListening
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) FinishListen(ctx context.Context, this TCPSocket) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateListening {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) Accept(ctx context.Context, this TCPSocket) witgo.Result[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateListening || sock.Listener == nil {
		return witgo.Err[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeInvalidState)
	}

	conn, err := sock.Listener.AcceptTCP()
	if err != nil {
		// 如果是超时或临时错误，映射到 would-block
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return witgo.Err[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](ErrorCodeWouldBlock)
		}
		return witgo.Err[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](mapOsError(err))
	}

	// 为新的连接创建一个新的 TCPSocket 资源
	newSock := &sockets.TCPSocket{
		Conn:   conn,
		Family: sock.Family,
		State:  sockets.TCPStateConnected,
	}
	newSockHandle := i.host.TCPSocketManager().Add(newSock)

	// 为新的连接创建输入输出流
	// conn 同时实现了 io.Reader 和 io.Writer
	inStream := manager_io.NewAsyncStreamForReader(conn, manager_io.DontCloseReader())
	inStreamHandle := i.host.StreamManager().Add(inStream)
	outStream := manager_io.NewAsyncStreamForWriter(conn, manager_io.DontCloseWriter())
	outStreamHandle := i.host.StreamManager().Add(outStream)

	result := witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream]{
		F0: newSockHandle,
		F1: inStreamHandle,  // F1 是 InputStream
		F2: outStreamHandle, // F2 是 OutputStream
	}

	return witgo.Ok[witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](result)
}

func (i *tcpImpl) Shutdown(ctx context.Context, this TCPSocket, shutdownType ShutdownType) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateConnected {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	var err error
	switch shutdownType {
	case ShutdownTypeReceive:
		err = sock.Conn.CloseRead()
	case ShutdownTypeSend:
		err = sock.Conn.CloseWrite()
	case ShutdownTypeBoth:
		err = sock.Conn.Close() // Close会同时关闭读和写
	default:
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) LocalAddress(ctx context.Context, this TCPSocket) witgo.Result[IPSocketAddress, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidArgument)
	}

	var addr net.Addr
	if sock.State >= sockets.TCPStateBound && sock.Listener != nil {
		addr = sock.Listener.Addr()
	} else if sock.State == sockets.TCPStateConnected && sock.Conn != nil {
		addr = sock.Conn.LocalAddr()
	} else {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidState)
	}

	wasiAddr, err := toIPSocketAddress(addr)
	if err != nil {
		return witgo.Err[IPSocketAddress, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[IPSocketAddress, ErrorCode](wasiAddr)
}

func (i *tcpImpl) RemoteAddress(ctx context.Context, this TCPSocket) witgo.Result[IPSocketAddress, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateConnected || sock.Conn == nil {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidState)
	}

	addr := sock.Conn.RemoteAddr()
	wasiAddr, err := toIPSocketAddress(addr)
	if err != nil {
		return witgo.Err[IPSocketAddress, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[IPSocketAddress, ErrorCode](wasiAddr)
}

func (i *tcpImpl) AddressFamily(ctx context.Context, this TCPSocket) IPAddressFamily {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return IPAddressFamilyIPV4 // or some other default/error indicator
	}
	family, _ := toIPAddressFamily(sock.Family)
	return family
}

func (i *tcpImpl) IsListening(ctx context.Context, this TCPSocket) bool {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return false
	}
	return sock.State == sockets.TCPStateListening
}

// SetKeepAliveEnabled 启用或禁用 keep-alive。
func (i *tcpImpl) SetKeepAliveEnabled(ctx context.Context, this TCPSocket, value bool) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		// 必须在连接建立后才能设置
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetKeepAlive(value)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// SetReceiveBufferSize 设置接收缓冲区大小。
func (i *tcpImpl) SetReceiveBufferSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetReadBuffer(int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// SetSendBufferSize 设置发送缓冲区大小。
func (i *tcpImpl) SetSendBufferSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetWriteBuffer(int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// Subscribe 创建一个 pollable 用于异步操作。
func (i *tcpImpl) Subscribe(pctx context.Context, this TCPSocket) wasip2_io.Pollable {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		// Invalid handle, return a ready pollable to allow the guest to discover the error quickly.
		p := manager_io.NewPollable(nil)
		handle := i.host.PollManager().Add(p)
		p.SetReady()
		return handle
	}

	// Handle different socket states appropriately.
	switch sock.State {
	case sockets.TCPStateConnecting:
		// For a connecting socket, the pollable must become ready when the non-blocking connect operation completes.
		ctx, cancel := context.WithCancel(pctx)
		p := manager_io.NewPollable(cancel)
		handle := i.host.PollManager().Add(p)

		// This goroutine waits for the connection result to appear on the channel.
		go func() {
			select {
			// Wait for the result from the connection goroutine.
			case res := <-sock.ConnectResult:
				// The connection attempt is complete. Signal the pollable.
				p.SetReady()
				// Put the result back onto the buffered channel so `finish-connect` can consume it.
				// This is a delicate operation, relying on the channel being buffered and having a single final consumer.
				sock.ConnectResult <- res
			case <-ctx.Done():
				// If the context is cancelled, also unblock the poll.
				p.SetReady()
			}
		}()
		return handle

	// case sockets.TCPStateListening, sockets.TCPStateConnected:
	// 	// For listening or connected sockets, we can poll on the underlying file descriptor if it exists.
	// 	if sock.Fd != 0 {
	// 		// For listening, poll for readability (new connection). For connected, poll for writability.
	// 		direction := manager_io.PollDirectionWrite
	// 		if sock.State == sockets.TCPStateListening {
	// 			direction = manager_io.PollDirectionRead
	// 		}
	// 		// NOTE: Assuming NewPollaleFd exists and works as intended for OS-specific polling.
	// 		p := manager_io.NewPollaleFd(sock.Fd, direction)
	// 		handle := i.host.PollManager().Add(p)
	// 		return handle
	// 	}
	// 	// Fallthrough if Fd is not available

	default:
		// For other states (e.g., unbound, closed), the operation should fail quickly.
		// A ready pollable allows the guest to proceed to the next step (e.g., finish-connect)
		// where it will receive the appropriate error for the socket's state.
		// p := manager_io.NewPollable(nil)
		// handle := i.host.PollManager().Add(p)
		// p.SetReady()
		// return handle
	}

	// Fallback for states with no specific polling mechanism (e.g. no Fd).
	p := manager_io.NewPollable(nil)
	handle := i.host.PollManager().Add(p)
	p.SetReady()
	return handle
}
