package v0_2

import (
	"context"
	"net"
	"syscall"
	"time"
	manager_io "wazero-wasip2/internal/io"
	"wazero-wasip2/internal/sockets"
	"wazero-wasip2/wasip2"
	wasip2_io "wazero-wasip2/wasip2/io/v0_2"
	witgo "wazero-wasip2/wit-go"
)

type tcpImpl struct {
	host *wasip2.Host
}

func newTCPImpl(h *wasip2.Host) *tcpImpl {
	return &tcpImpl{host: h}
}

func (i *tcpImpl) DropTCPSocket(_ context.Context, handle TCPSocket) {
	sock, ok := i.host.TCPSocketManager().Get(handle)
	if !ok {
		return
	}
	if sock.Conn != nil {
		sock.Conn.Close()
	}
	if sock.Listener != nil {
		sock.Listener.Close()
	}
	i.host.TCPSocketManager().Remove(handle)
}

func (i *tcpImpl) StartBind(_ context.Context, this TCPSocket, network Network, localAddress IPSocketAddress) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateUnbound {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	addr, err := fromIPSocketAddress(localAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	// 在 Go 中，bind 和 listen 是一起通过 net.ListenTCP 完成的。
	// 但 WASI 将它们分开。这里我们只做 bind 的准备工作。
	// 为了简化，我们暂时将地址存储起来，在 listen 时使用。
	// 一个更完整的实现会使用 syscall.Bind。

	// 这里我们直接尝试 Listen，如果成功，就认为 bind 成功了。
	// 这不完全符合 WASI 的异步模型，但对于一个简单的实现是可行的。
	listener, listenErr := net.ListenTCP("tcp", addr.(*net.TCPAddr))
	if listenErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(listenErr))
	}

	// 绑定成功，更新套接字状态
	sock.Listener = listener
	sock.State = sockets.TCPStateBound

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) FinishBind(_ context.Context, this TCPSocket) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	// 在我们的简化模型中，start-bind 已经完成了所有工作。
	if sock.State != sockets.TCPStateBound {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
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

	addr, err := fromIPSocketAddress(remoteAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	sock.State = sockets.TCPStateConnecting
	sock.ConnectResult = make(chan sockets.ConnectResult, 1) // 创建带缓冲的 channel

	// 在后台 goroutine 中执行阻塞的 Dial 操作
	go func() {
		// net.DialTCP 会处理隐式绑定（如果套接字未绑定）
		conn, dialErr := net.DialTCP("tcp", nil, addr.(*net.TCPAddr))
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
		stream := &manager_io.Stream{Reader: sock.Conn, Writer: sock.Conn, Closer: sock.Conn}
		streamHandle := i.host.StreamManager().Add(stream)

		return witgo.Ok[witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream], ErrorCode](witgo.Tuple[wasip2_io.InputStream, wasip2_io.OutputStream]{
			F0: streamHandle,
			F1: streamHandle,
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
	stream := &manager_io.Stream{Reader: conn, Writer: conn, Closer: conn}
	streamHandle := i.host.StreamManager().Add(stream)

	result := witgo.Tuple3[TCPSocket, wasip2_io.InputStream, wasip2_io.OutputStream]{
		F0: newSockHandle,
		F1: streamHandle, // F1 是 InputStream
		F2: streamHandle, // F2 是 OutputStream
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

func (i *tcpImpl) SetListenBacklogSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	// Go 的 net.ListenTCP 有一个 backlog 参数，但一旦开始监听就无法更改。
	// 这个操作在 Go 中通常不支持。
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

// KeepAliveEnabled 查询 keep-alive 是否启用。
func (i *tcpImpl) KeepAliveEnabled(ctx context.Context, this TCPSocket) witgo.Result[bool, ErrorCode] {
	// Go 标准库没有提供此选项的 getter 方法。我们默认假设为 false。
	// 一个完整的实现需要使用 syscall 来获取 SO_KEEPALIVE 选项。
	return witgo.Ok[bool, ErrorCode](false) // 暂时返回占位符
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

// KeepAliveIdleTime 查询 TCP keep-alive 的空闲时间。
func (i *tcpImpl) KeepAliveIdleTime(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	// 获取 TCP_KEEPIDLE 需要 syscall。
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

// SetKeepAliveIdleTime 设置 TCP keep-alive 的空闲时间。
// 注意：Go 的 SetKeepAlivePeriod 同时影响 idle time 和 interval，这与 POSIX 有所不同。
func (i *tcpImpl) SetKeepAliveIdleTime(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetKeepAlivePeriod(time.Duration(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// KeepAliveInterval 查询 TCP keep-alive 的间隔时间。
func (i *tcpImpl) KeepAliveInterval(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	// 获取 TCP_KEEPINTVL 需要 syscall。
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

// SetKeepAliveInterval 设置 TCP keep-alive 的间隔时间。
func (i *tcpImpl) SetKeepAliveInterval(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

// KeepAliveCount 查询 TCP keep-alive 的探测次数。
func (i *tcpImpl) KeepAliveCount(ctx context.Context, this TCPSocket) witgo.Result[uint32, ErrorCode] {
	// 获取 TCP_KEEPCNT 需要 syscall。
	return witgo.Err[uint32, ErrorCode](ErrorCodeNotSupported)
}

// SetKeepAliveCount 设置 TCP keep-alive 的探测次数。
func (i *tcpImpl) SetKeepAliveCount(ctx context.Context, this TCPSocket, value uint32) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

// HopLimit 查询单播的跳数限制 (TTL)。
func (i *tcpImpl) HopLimit(ctx context.Context, this TCPSocket) witgo.Result[uint8, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[uint8, ErrorCode](ErrorCodeInvalidArgument)
	}

	rawConn, err := sock.Conn.SyscallConn()
	if err != nil {
		return witgo.Err[uint8, ErrorCode](mapOsError(err))
	}

	var ttl int
	var getErr error
	err = rawConn.Control(func(fd uintptr) {
		if sock.Family == sockets.IPAddressFamilyIPV4 {
			ttl, getErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL)
		} else {
			ttl, getErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS)
		}
	})

	if err != nil {
		return witgo.Err[uint8, ErrorCode](mapOsError(err))
	}
	if getErr != nil {
		return witgo.Err[uint8, ErrorCode](mapOsError(getErr))
	}

	return witgo.Ok[uint8, ErrorCode](uint8(ttl))
}

// SetHopLimit 设置单播的跳数限制 (TTL)。
func (i *tcpImpl) SetHopLimit(ctx context.Context, this TCPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	rawConn, err := sock.Conn.SyscallConn()
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}

	var setErr error
	err = rawConn.Control(func(fd uintptr) {
		if sock.Family == sockets.IPAddressFamilyIPV4 {
			setErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, int(value))
		} else {
			setErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS, int(value))
		}
	})

	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	if setErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(setErr))
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// ReceiveBufferSize 查询接收缓冲区大小。
func (i *tcpImpl) ReceiveBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
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

// SendBufferSize 查询发送缓冲区大小。
func (i *tcpImpl) SendBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
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
func (i *tcpImpl) Subscribe(ctx context.Context, this TCPSocket) wasip2_io.Pollable {
	p := manager_io.NewPollable(nil)
	handle := i.host.PollManager().Add(p)

	_, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		p.SetReady()
		return handle
	}

	// 在一个真实的异步实现中，一个后台 goroutine 会监控套接字的文件描述符
	// 的就绪状态，并在适当时调用 p.SetReady()。
	// 目前，我们立即将其设为就绪，以允许轮询循环继续进行。
	p.SetReady()

	return handle
}
