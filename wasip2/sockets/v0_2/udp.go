package v0_2

import (
	"context"

	manager_io "github.com/foxxorcat/wazero-wasip2/manager/io"
	manager_sockets "github.com/foxxorcat/wazero-wasip2/manager/sockets"

	"github.com/foxxorcat/wazero-wasip2/wasip2"
	wasip2_io "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type udpImpl struct {
	host *wasip2.Host
}

func newUDPImpl(h *wasip2.Host) *udpImpl {
	return &udpImpl{host: h}
}

func (i *udpImpl) Stream(_ context.Context, this UDPSocket, remoteAddress witgo.Option[IPSocketAddress]) witgo.Result[witgo.Tuple[IncomingDatagramStream, OutgoingDatagramStream], ErrorCode] {
	// stream 方法用于创建收发数据报的流。在我们的实现中，
	// incoming 和 outgoing stream 将共享同一个 UDP socket 资源。
	// 我们返回相同的句柄作为两个流。
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[witgo.Tuple[IncomingDatagramStream, OutgoingDatagramStream], ErrorCode](ErrorCodeInvalidState)
	}

	if sock.Reader != nil {
		sock.Reader.Close()
	}
	sock.Reader = manager_sockets.NewAsyncUDPReader(sock.Conn)
	if sock.Writer != nil {
		sock.Writer.Close()
	}
	sock.Writer = manager_sockets.NewAsyncUDPWriter(sock.Conn)

	return witgo.Ok[witgo.Tuple[IncomingDatagramStream, OutgoingDatagramStream], ErrorCode](
		witgo.Tuple[IncomingDatagramStream, OutgoingDatagramStream]{
			F0: this,
			F1: this,
		},
	)
}

func (i *udpImpl) LocalAddress(ctx context.Context, this UDPSocket) witgo.Result[IPSocketAddress, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidState)
	}
	addr, err := toIPSocketAddress(sock.Conn.LocalAddr())
	if err != nil {
		return witgo.Err[IPSocketAddress, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[IPSocketAddress, ErrorCode](addr)
}

func (i *udpImpl) RemoteAddress(ctx context.Context, this UDPSocket) witgo.Result[IPSocketAddress, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidState)
	}
	addr, err := toIPSocketAddress(sock.Conn.RemoteAddr())
	if err != nil {
		return witgo.Err[IPSocketAddress, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[IPSocketAddress, ErrorCode](addr)
}

func (i *udpImpl) AddressFamily(ctx context.Context, this UDPSocket) IPAddressFamily {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok {
		return IPAddressFamilyIPV4
	}
	family, _ := toIPAddressFamily(sock.Family)
	return family
}

// SetReceiveBufferSize 设置接收缓冲区大小。
func (i *udpImpl) SetReceiveBufferSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetReadBuffer(int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

// SetSendBufferSize 设置发送缓冲区大小。
func (i *udpImpl) SetSendBufferSize(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	sock, ok := i.host.TCPSocketManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidArgument)
	}
	if sock.Conn == nil {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeInvalidState)
	}
	err := sock.Conn.SetWriteBuffer(int(value))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *udpImpl) Subscribe(ctx context.Context, this UDPSocket) wasip2_io.Pollable {
	// sock, ok := i.host.TCPSocketManager().Get(this)
	// if !ok || sock.Fd == 0 {
	// 	return i.host.PollManager().Add(manager_io.ReadyPollable)
	// }
	// UDP 没有链接状态
	return i.host.PollManager().Add(manager_io.ReadyPollable)
}

func (i *udpImpl) DropIncomingDatagramStream(_ context.Context, handle IncomingDatagramStream) {
	sock, ok := i.host.UDPSocketManager().Get(handle)
	if ok && sock.Reader != nil {
		sock.Reader.Close()
		sock.Writer = nil
	}
}

func (i *udpImpl) DropOutgoingDatagramStream(_ context.Context, handle OutgoingDatagramStream) {
	sock, ok := i.host.UDPSocketManager().Get(handle)
	if ok && sock.Writer != nil {
		sock.Writer.Close()
		sock.Writer = nil
	}
}

func (i *udpImpl) Receive(_ context.Context, this IncomingDatagramStream, maxResults uint64) witgo.Result[[]IncomingDatagram, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Reader == nil {
		return witgo.Err[[]IncomingDatagram, ErrorCode](ErrorCodeInvalidArgument)
	}

	if maxResults == 0 {
		return witgo.Ok[[]IncomingDatagram, ErrorCode]([]IncomingDatagram{})
	}

	datagrams, err := sock.Reader.Receive(maxResults)
	if err != nil {
		return witgo.Err[[]IncomingDatagram, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[[]IncomingDatagram, ErrorCode](datagrams)
}

func (i *udpImpl) Send(_ context.Context, this OutgoingDatagramStream, datagrams []OutgoingDatagram) witgo.Result[uint64, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Writer == nil {
		return witgo.Err[uint64, ErrorCode](ErrorCodeInvalidArgument)
	}

	sentCount, err := sock.Writer.Send(datagrams)
	if err != nil {
		return witgo.Err[uint64, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[uint64, ErrorCode](sentCount)
}

func (i *udpImpl) CheckSend(_ context.Context, this OutgoingDatagramStream) witgo.Result[uint64, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Writer == nil {
		// As per spec, invalid-state is not a possible error. We return 0.
		return witgo.Ok[uint64, ErrorCode](4096)
	}
	available := sock.Writer.AvailableSpace()
	return witgo.Ok[uint64, ErrorCode](available)
}

func (i *udpImpl) SubscribeIncoming(ctx context.Context, this IncomingDatagramStream) wasip2_io.Pollable {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Reader == nil {
		return i.host.PollManager().Add(manager_io.ReadyPollable)
	}
	return i.host.PollManager().Add(sock.Reader.Subscribe())
}

func (i *udpImpl) SubscribeOutgoing(ctx context.Context, this OutgoingDatagramStream) wasip2_io.Pollable {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Writer == nil {
		return i.host.PollManager().Add(manager_io.ReadyPollable)
	}
	return i.host.PollManager().Add(sock.Writer.Subscribe())
}
