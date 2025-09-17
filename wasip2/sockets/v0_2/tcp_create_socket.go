package v0_2

import (
	"context"
	"syscall"
	"wazero-wasip2/internal/sockets"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"
)

type tcpCreateSocketImpl struct {
	host *wasip2.Host
}

func newTCPCreateSocketImpl(h *wasip2.Host) *tcpCreateSocketImpl {
	return &tcpCreateSocketImpl{host: h}
}

func (i *tcpCreateSocketImpl) CreateTCPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[TCPSocket, ErrorCode] {
	// wasi:sockets 的一个关键点是，创建套接字本身并不需要网络权限。
	// 它只是一个内存中的对象，直到 bind 或 connect 才与网络交互。

	// 在 Go 中，我们无法在不实际创建系统套接字的情况下模拟一个套接字。
	// 所以我们在这里就创建它，但它处于未绑定、未连接的状态。

	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[TCPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	// 这里我们直接使用 syscall 来创建套接字，以便更好地控制。
	// syscall.Socket 的行为在不同平台（特别是 Windows）上有差异，
	// 一个完整的实现需要处理这些差异。
	domain := syscall.AF_INET
	if family == sockets.IPAddressFamilyIPV6 {
		domain = syscall.AF_INET6
	}

	// 非阻塞是 WASI sockets 的默认行为
	sockFd, sockErr := syscall.Socket(domain, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK, syscall.IPPROTO_TCP)
	if sockErr != nil {
		return witgo.Err[TCPSocket, ErrorCode](mapOsError(sockErr))
	}

	// 在 IPv6 套接字上默认启用 IPV6_V6ONLY
	if family == sockets.IPAddressFamilyIPV6 {
		syscall.SetsockoptInt(sockFd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}

	// 将 syscall 的文件描述符转换为 Go 的 *net.TCPConn
	// 这部分逻辑较为复杂，一个简化的方法是暂时不创建 net.Conn，
	// 仅保存 fd，在 bind/connect 时再处理。
	// 为了简化，我们暂时创建一个空的 TCPSocket 结构。

	tcpSocket := &sockets.TCPSocket{
		Family: family,
		State:  sockets.TCPStateUnbound,
		Fd:     sockFd, // 可以选择保存原始 fd
	}

	handle := i.host.TCPSocketManager().Add(tcpSocket)
	return witgo.Ok[TCPSocket, ErrorCode](handle)
}
