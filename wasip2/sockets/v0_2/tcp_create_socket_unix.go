//go:build unix

package v0_2

import (
	"context"
	"wazero-wasip2/internal/sockets"
	witgo "wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *tcpCreateSocketImpl) CreateTCPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[TCPSocket, ErrorCode] {
	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[TCPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	domain := unix.AF_INET
	if family == IPAddressFamilyIPV6 {
		domain = unix.AF_INET6
	}

	// On Darwin, SOCK_NONBLOCK is not available, so we create a standard socket first.
	sockFd, sockErr := unix.Socket(domain, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if sockErr != nil {
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(sockErr))
	}

	// Then, we use fcntl to set the O_NONBLOCK flag to make it non-blocking.
	err = unix.SetNonblock(sockFd, true)
	if err != nil {
		unix.Close(sockFd)
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(err))
	}

	// 默认启用 IPV6_V6ONLY 以符合现代网络实践
	if family == IPAddressFamilyIPV6 {
		// 忽略此处的错误是安全的，因为并非所有系统都支持此选项
		unix.SetsockoptInt(sockFd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 1)
	}

	tcpSocket := &sockets.TCPSocket{
		Family: family,
		State:  sockets.TCPStateUnbound,
		Fd:     sockFd,
	}

	handle := i.host.TCPSocketManager().Add(tcpSocket)
	return witgo.Ok[TCPSocket, ErrorCode](handle)
}
