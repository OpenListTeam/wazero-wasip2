//go:build !unix && !windows

package v0_2

import (
	"context"
	"wazero-wasip2/internal/sockets"
	witgo "wazero-wasip2/wit-go"
)

func (i *udpCreateSocketImpl) CreateUDPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[TCPSocket, ErrorCode] {
	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[UDPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	// 此阶段我们不将其转换为 net.UDPConn，因为 Go 的 net.FileConn 会改变非阻塞状态。
	// 我们仅保存文件描述符，在 bind/connect/stream 时再处理。
	udpSocket := &sockets.UDPSocket{
		Family: family,
	}

	handle := i.host.UDPSocketManager().Add(udpSocket)
	return witgo.Ok[UDPSocket, ErrorCode](handle)
}

func (i *udpImpl) DropUDPSocket(_ context.Context, handle UDPSocket) {
	i.host.UDPSocketManager().Remove(handle)
}
