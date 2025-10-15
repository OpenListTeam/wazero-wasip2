//go:build windows

package v0_2

import (
	"context"
	"unsafe"

	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"golang.org/x/sys/windows"
)

// FIONBIO is the command to set non-blocking I/O mode on a socket.
// This constant is defined here to avoid dependency on a specific version of the x/sys/windows package.
const FIONBIO = 0x8004667e

func (i *tcpCreateSocketImpl) CreateTCPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[TCPSocket, ErrorCode] {
	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[TCPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	domain := windows.AF_INET
	if family == sockets.IPAddressFamilyIPV6 {
		domain = windows.AF_INET6
	}

	handle, err := windows.Socket(domain, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(err))
	}

	// 使用 WSAIoctl 将套接字设置为非阻塞模式
	var nonBlockingMode uint32 = 1
	var bytesReturned uint32
	err = windows.WSAIoctl(
		handle,
		FIONBIO,
		(*byte)(unsafe.Pointer(&nonBlockingMode)),
		uint32(unsafe.Sizeof(nonBlockingMode)),
		nil,
		0,
		&bytesReturned,
		nil,
		0,
	)
	if err != nil {
		windows.Closesocket(handle)
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(err))
	}

	if family == sockets.IPAddressFamilyIPV6 {
		val := 1
		err = windows.SetsockoptInt(handle, windows.IPPROTO_IPV6, windows.IPV6_V6ONLY, val)
		if err != nil {
			windows.Closesocket(handle)
			return witgo.Err[TCPSocket, ErrorCode](mapOsError(err))
		}
	}

	tcpSocket := &sockets.TCPSocket{
		Family: family,
		State:  sockets.TCPStateUnbound,
		Fd:     int(handle),
	}

	h := i.host.TCPSocketManager().Add(tcpSocket)
	return witgo.Ok[TCPSocket, ErrorCode](h)
}
