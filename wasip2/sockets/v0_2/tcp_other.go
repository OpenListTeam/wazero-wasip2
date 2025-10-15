//go:build !unix && !windows

package v0_2

import (
	"context"
	"net"

	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

func (i *tcpImpl) DropTCPSocket(_ context.Context, handle TCPSocket) {
	sock, ok := i.host.TCPSocketManager().Get(handle)
	if !ok {
		return
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

	addr, err := fromIPSocketAddressToTCPAddr(localAddress)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}

	pnetwork := "tcp"
	if sock.Family == sockets.IPAddressFamilyIPV6 {
		pnetwork = "tcp6"
	}

	listener, listenErr := net.ListenTCP(pnetwork, addr)
	if listenErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(listenErr))
	}

	// 绑定成功，更新套接字状态
	sock.Listener = listener
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
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) KeepAliveEnabled(ctx context.Context, this TCPSocket) witgo.Result[bool, ErrorCode] {
	return witgo.Err[bool, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) KeepAliveIdleTime(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) SetKeepAliveIdleTime(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) KeepAliveInterval(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) SetKeepAliveInterval(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) KeepAliveCount(ctx context.Context, this TCPSocket) witgo.Result[uint32, ErrorCode] {
	return witgo.Err[uint32, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) SetKeepAliveCount(ctx context.Context, this TCPSocket, value uint32) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) HopLimit(ctx context.Context, this TCPSocket) witgo.Result[uint8, ErrorCode] {
	return witgo.Err[uint8, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) SetHopLimit(ctx context.Context, this TCPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) ReceiveBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}

func (i *tcpImpl) SendBufferSize(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}
