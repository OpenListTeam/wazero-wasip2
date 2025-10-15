//go:build !unix && !windows

package v0_2

import (
	"context"

	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

func (i *tcpCreateSocketImpl) CreateTCPSocket(_ context.Context, addressFamily IPAddressFamily) witgo.Result[TCPSocket, ErrorCode] {
	family, err := fromIPAddressFamily(addressFamily)
	if err != nil {
		return witgo.Err[TCPSocket, ErrorCode](ErrorCodeNotSupported)
	}

	tcpSocket := &sockets.TCPSocket{
		Family: family,
		State:  sockets.TCPStateUnbound,
	}

	handle := i.host.TCPSocketManager().Add(tcpSocket)
	return witgo.Ok[TCPSocket, ErrorCode](handle)
}
