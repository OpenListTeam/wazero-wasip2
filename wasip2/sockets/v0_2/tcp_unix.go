//go:build unix

package v0_2

import (
	"context"
	"net"
	"os"
	"syscall"

	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *tcpImpl) DropTCPSocket(_ context.Context, handle TCPSocket) {
	sock, ok := i.host.TCPSocketManager().Get(handle)
	if !ok {
		return
	}
	if sock.Fd != 0 {
		unix.Close(sock.Fd)
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

	sockaddr, err := fromIPSocketAddressToSockaddr(localAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	// 执行 syscall.Bind
	bindErr := syscall.Bind(int(sock.Fd), sockaddr)
	if bindErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(bindErr))
	}

	file := os.NewFile(uintptr(sock.Fd), "")
	listener, connErr := net.FileListener(file)
	if connErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(connErr))
	}

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

	// This is not directly supported by Go's net.Listener, so we use syscall.
	file, err := sock.Listener.File()
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	defer file.Close()

	err = unix.Listen(int(file.Fd()), int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *tcpImpl) KeepAliveEnabled(ctx context.Context, this TCPSocket) witgo.Result[bool, ErrorCode] {
	result := getsockoptInt[int](i, this, unix.SOL_SOCKET, unix.SO_KEEPALIVE)
	if result.Err != nil {
		return witgo.Err[bool, ErrorCode](*result.Err)
	}
	return witgo.Ok[bool, ErrorCode](*result.Ok == 1)
}

func (i *tcpImpl) KeepAliveInterval(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getsockoptInt[uint64](i, this, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL)
}

func (i *tcpImpl) SetKeepAliveInterval(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return setsockoptInt(i, this, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, int(value))
}

func (i *tcpImpl) KeepAliveCount(ctx context.Context, this TCPSocket) witgo.Result[uint32, ErrorCode] {
	return getsockoptInt[uint32](i, this, unix.IPPROTO_TCP, unix.TCP_KEEPCNT)
}

func (i *tcpImpl) SetKeepAliveCount(ctx context.Context, this TCPSocket, value uint32) witgo.Result[witgo.Unit, ErrorCode] {
	return setsockoptInt(i, this, unix.IPPROTO_TCP, unix.TCP_KEEPCNT, int(value))
}

func (i *tcpImpl) HopLimit(ctx context.Context, this TCPSocket) witgo.Result[uint8, ErrorCode] {
	return getsockoptInt[uint8](i, this, unix.IPPROTO_IP, unix.IP_TTL)
}

func (i *tcpImpl) SetHopLimit(ctx context.Context, this TCPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return setsockoptInt(i, this, unix.IPPROTO_IP, unix.IP_TTL, int(value))
}

func (i *tcpImpl) ReceiveBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getsockoptInt[uint64](i, this, unix.SOL_SOCKET, unix.SO_RCVBUF)
}

func (i *tcpImpl) SendBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getsockoptInt[uint64](i, this, unix.SOL_SOCKET, unix.SO_SNDBUF)
}

func getsockoptInt[T ~int | ~uint64 | ~uint32 | ~uint8](i *tcpImpl, this TCPSocket, level, opt int) witgo.Result[T, ErrorCode] {
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
		val, getErr = unix.GetsockoptInt(int(fd), level, opt)
	})

	if err != nil {
		return witgo.Err[T, ErrorCode](mapOsError(err))
	}
	if getErr != nil {
		return witgo.Err[T, ErrorCode](mapOsError(getErr))
	}

	return witgo.Ok[T, ErrorCode](T(val))
}

func setsockoptInt(i *tcpImpl, this TCPSocket, level, opt, value int) witgo.Result[witgo.Unit, ErrorCode] {
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
		setErr = unix.SetsockoptInt(int(fd), level, opt, value)
	})

	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	if setErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(setErr))
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
