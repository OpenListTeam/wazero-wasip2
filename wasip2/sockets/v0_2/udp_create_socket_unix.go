//go:build unix

package v0_2

import (
	"context"
	"syscall"
	"wazero-wasip2/internal/sockets"
	witgo "wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *udpCreateSocketImpl) CreateUDPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[UDPSocket, ErrorCode] {
	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[UDPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	domain := syscall.AF_INET
	if family == sockets.IPAddressFamilyIPV6 {
		domain = syscall.AF_INET6
	}

	// 创建一个非阻塞的 UDP 系统套接字
	sockFd, sockErr := syscall.Socket(domain, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if sockErr != nil {
		return witgo.Err[UDPSocket, ErrorCode](mapOsError(sockErr))
	}

	// Then, we use fcntl to set the O_NONBLOCK flag to make it non-blocking.
	err = unix.SetNonblock(sockFd, true)
	if err != nil {
		unix.Close(sockFd)
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(err))
	}

	if family == sockets.IPAddressFamilyIPV6 {
		// 确保 IPv6 套接字只用于 IPv6
		syscall.SetsockoptInt(sockFd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}

	// 此阶段我们不将其转换为 net.UDPConn，因为 Go 的 net.FileConn 会改变非阻塞状态。
	// 我们仅保存文件描述符，在 bind/connect/stream 时再处理。
	udpSocket := &sockets.UDPSocket{
		Fd:     sockFd,
		Family: family,
	}

	handle := i.host.UDPSocketManager().Add(udpSocket)
	return witgo.Ok[UDPSocket, ErrorCode](handle)
}

func (i *udpImpl) DropUDPSocket(_ context.Context, handle UDPSocket) {
	sock, ok := i.host.UDPSocketManager().Get(handle)
	if !ok {
		return
	}
	if sock.Conn != nil {
		sock.Conn.Close()
	} else if sock.Fd != 0 {
		unix.Close(sock.Fd)
	}
	i.host.UDPSocketManager().Remove(handle)
}
