//go:build windows

package v0_2

import (
	"context"
	"net"
	"os"
	"syscall"

	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

	"golang.org/x/sys/windows"
)

func (i *udpImpl) StartBind(_ context.Context, this UDPSocket, network Network, localAddress IPSocketAddress) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	// 将 WIT 地址转换为 syscall.Sockaddr
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
	conn, connErr := net.FileConn(file)
	if connErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(connErr))
	}
	sock.Conn = conn.(*net.UDPConn)

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *udpImpl) FinishBind(_ context.Context, this UDPSocket) witgo.Result[witgo.Unit, ErrorCode] {
	// 我们的 start-bind 是同步的，所以这里直接成功返回
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *udpImpl) UnicastHopLimit(ctx context.Context, this UDPSocket) witgo.Result[uint8, ErrorCode] {
	return getUDPSockoptInt[uint8](i, this, windows.IPPROTO_IP, windows.IP_TTL)
}

func (i *udpImpl) SetUnicastHopLimit(ctx context.Context, this UDPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return setUDPSockoptInt(i, this, windows.IPPROTO_IP, windows.IP_TTL, int(value))
}

func (i *udpImpl) ReceiveBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return getUDPSockoptInt[uint64](i, this, windows.SOL_SOCKET, windows.SO_RCVBUF)
}

func (i *udpImpl) SendBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return getUDPSockoptInt[uint64](i, this, windows.SOL_SOCKET, windows.SO_SNDBUF)
}

func getUDPSockoptInt[T ~int | ~uint64 | ~uint32 | ~uint8](i *udpImpl, this UDPSocket, level, opt int) witgo.Result[T, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
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

func setUDPSockoptInt(i *udpImpl, this UDPSocket, level, opt, value int) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
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
