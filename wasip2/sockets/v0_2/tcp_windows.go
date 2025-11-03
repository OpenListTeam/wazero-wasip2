//go:build windows

package v0_2

import (
	"context"
	"net"
	"os"
	"syscall"

	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"golang.org/x/sys/windows"
)

func (i *tcpImpl) StartBind(_ context.Context, this TCPSocket, network Network, localAddress IPSocketAddress) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.State != sockets.TCPStateUnbound {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	sockaddr, err := fromIPSocketAddressToSockaddr(localAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	// 执行 syscall.Bind
	bindErr := syscall.Bind(syscall.Handle(sock.Fd), sockaddr)
	if bindErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(bindErr))
	}

	// 绑定成功后，将 Fd 转换为 net.UDPConn 以便后续操作
	file := os.NewFile(uintptr(sock.Fd), "")
	listener, connErr := net.FileListener(file)
	if connErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(connErr))
	}

	// 绑定成功，更新套接字状态
	sock.Listener = listener.(*net.TCPListener)
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

func (i *tcpImpl) SetListenBacklogSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Listener == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}

	file, err := sock.Listener.File()
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	defer file.Close()

	err = windows.Listen(windows.Handle(file.Fd()), int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) KeepAliveEnabled(ctx context.Context, this TCPSocket) witgo.Result[bool, ErrorCode] {
	result := getTCPSockopt[int](i, this, windows.SOL_SOCKET, windows.SO_KEEPALIVE)
	if result.Err != nil {
		return witgo.Err[bool, ErrorCode](*result.Err)
	}
	return witgo.Ok[bool, ErrorCode](*result.Ok == 1)
}

func (i *tcpImpl) KeepAliveIdleTime(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getTCPSockopt[uint64](i, this, windows.IPPROTO_TCP, windows.TCP_KEEPIDLE)
}

func (i *tcpImpl) SetKeepAliveIdleTime(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return setTCPSockopt(i, this, windows.IPPROTO_TCP, windows.TCP_KEEPIDLE, int(value))
}

func (i *tcpImpl) KeepAliveInterval(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getTCPSockopt[uint64](i, this, windows.IPPROTO_TCP, windows.TCP_KEEPINTVL)
}

func (i *tcpImpl) SetKeepAliveInterval(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return setTCPSockopt(i, this, windows.IPPROTO_TCP, windows.TCP_KEEPINTVL, int(value))
}

func (i *tcpImpl) KeepAliveCount(ctx context.Context, this TCPSocket) witgo.Result[uint32, ErrorCode] {
	return getTCPSockopt[uint32](i, this, windows.IPPROTO_TCP, windows.TCP_KEEPCNT)
}

func (i *tcpImpl) SetKeepAliveCount(ctx context.Context, this TCPSocket, value uint32) witgo.Result[witgo.Unit, ErrorCode] {
	return setTCPSockopt(i, this, windows.IPPROTO_TCP, windows.TCP_KEEPCNT, int(value))
}

func (i *tcpImpl) HopLimit(ctx context.Context, this TCPSocket) witgo.Result[uint8, ErrorCode] {
	return getTCPSockopt[uint8](i, this, windows.IPPROTO_IP, windows.IP_TTL)
}

func (i *tcpImpl) SetHopLimit(ctx context.Context, this TCPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return setTCPSockopt(i, this, windows.IPPROTO_IP, windows.IP_TTL, int(value))
}

func (i *tcpImpl) ReceiveBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getTCPSockopt[uint64](i, this, windows.SOL_SOCKET, windows.SO_RCVBUF)
}

func (i *tcpImpl) SendBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getTCPSockopt[uint64](i, this, windows.SOL_SOCKET, windows.SO_SNDBUF)
}

func getTCPSockopt[T ~int | ~uint64 | ~uint32 | ~uint8](i *tcpImpl, this TCPSocket, level, opt int) witgo.Result[T, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[T, ErrorCode](ErrorCodeInvalidArgument)
	}

	rawConn, err := sock.Conn.SyscallConn()
	if err != nil {
		return witgo.Err[T, ErrorCode](mapOsError(err))
	}

	var val int
	var getErr error
	err = rawConn.Control(func(fd uintptr) {
		val, getErr = windows.GetsockoptInt(windows.Handle(fd), level, opt)
	})

	if err != nil {
		return witgo.Err[T, ErrorCode](mapOsError(err))
	}
	if getErr != nil {
		return witgo.Err[T, ErrorCode](mapOsError(getErr))
	}

	return witgo.Ok[T, ErrorCode](T(val))
}

func setTCPSockopt(i *tcpImpl, this TCPSocket, level, opt, value int) witgo.Result[witgo.Unit, ErrorCode] {
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
		setErr = windows.SetsockoptInt(windows.Handle(fd), level, opt, value)
	})

	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	if setErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(setErr))
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
