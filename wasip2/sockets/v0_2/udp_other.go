//go:build !unix && !windows

package v0_2

import (
	"context"
	"net"

	"github.com/foxxorcat/wazero-wasip2/manager/sockets"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

func (i *udpImpl) StartBind(_ context.Context, this UDPSocket, network Network, localAddress IPSocketAddress) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	addr, err := fromIPSocketAddressToUDPAddr(localAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	pnetwork := "udp"
	if sock.Family == sockets.IPAddressFamilyIPV6 {
		pnetwork = "udp6"
	}

	conn, listenErr := net.ListenUDP(pnetwork, addr)
	if listenErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(listenErr))
	}

	// 绑定成功，更新套接字状态
	sock.Conn = conn

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *udpImpl) FinishBind(_ context.Context, this UDPSocket) witgo.Result[witgo.Unit, ErrorCode] {
	// 我们的 start-bind 是同步的，所以这里直接成功返回
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *udpImpl) UnicastHopLimit(ctx context.Context, this UDPSocket) witgo.Result[uint8, ErrorCode] {
	return witgo.Err[uint8, ErrorCode](ErrorCodeNotSupported)
}

func (i *udpImpl) SetUnicastHopLimit(ctx context.Context, this UDPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *udpImpl) ReceiveBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

func (i *udpImpl) SendBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}
