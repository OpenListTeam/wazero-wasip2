package v0_2

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	manager_io "wazero-wasip2/internal/io"
	"wazero-wasip2/wasip2"
	wasip2_io "wazero-wasip2/wasip2/io/v0_2"
	witgo "wazero-wasip2/wit-go"
)

type udpImpl struct {
	host *wasip2.Host
}

func newUDPImpl(h *wasip2.Host) *udpImpl {
	return &udpImpl{host: h}
}

func (i *udpImpl) DropUDPSocket(_ context.Context, handle UDPSocket) {
	sock, ok := i.host.UDPSocketManager().Get(handle)
	if !ok {
		return
	}
	if sock.Conn != nil {
		sock.Conn.Close()
	} else if sock.Fd != 0 {
		syscall.Close(sock.Fd)
	}
	i.host.UDPSocketManager().Remove(handle)
}

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
	bindErr := syscall.Bind(sock.Fd, sockaddr)
	if bindErr != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(bindErr))
	}

	// 绑定成功后，将 Fd 转换为 net.UDPConn 以便后续操作
	file := os.NewFile(uintptr(sock.Fd), "")
	conn, connErr := net.FileConn(file)
	file.Close() // FileConn 会复制 fd，所以可以关闭原始的
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

func (i *udpImpl) Stream(_ context.Context, this UDPSocket, remoteAddress witgo.Option[IPSocketAddress]) witgo.Result[witgo.Tuple[IncomingDatagramStream, OutgoingDatagramStream], ErrorCode] {
	// stream 方法用于创建收发数据报的流。在我们的实现中，
	// incoming 和 outgoing stream 将共享同一个 UDP socket 资源。
	// 我们返回相同的句柄作为两个流。
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
	// UDP `connect` 之后才有 remote address
	return witgo.Err[IPSocketAddress, ErrorCode](ErrorCodeInvalidState)
}

func (i *udpImpl) AddressFamily(ctx context.Context, this UDPSocket) IPAddressFamily {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok {
		return IPAddressFamilyIPV4
	}
	family, _ := toIPAddressFamily(sock.Family)
	return family
}

// ... 其他 UDP socket option 方法的存根 ...
func (i *udpImpl) UnicastHopLimit(ctx context.Context, this UDPSocket) witgo.Result[uint8, ErrorCode] {
	return witgo.Err[uint8, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) SetUnicastHopLimit(ctx context.Context, this UDPSocket, value uint8) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) ReceiveBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) SetReceiveBufferSize(ctx context.Context, this UDPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) SendBufferSize(ctx context.Context, this UDPSocket) witgo.Result[uint64, ErrorCode] {
	return witgo.Err[uint64, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) SetSendBufferSize(ctx context.Context, this UDPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeNotSupported)
}
func (i *udpImpl) Subscribe(ctx context.Context, this UDPSocket) wasip2_io.Pollable {
	p := manager_io.NewPollable(nil)
	p.SetReady()
	return i.host.PollManager().Add(p)
}

// --- Datagram Stream Implementations ---
// --- Datagram Stream Implementations ---

func (i *udpImpl) DropIncomingDatagramStream(_ context.Context, handle IncomingDatagramStream) {}
func (i *udpImpl) DropOutgoingDatagramStream(_ context.Context, handle OutgoingDatagramStream) {}

func (i *udpImpl) Receive(_ context.Context, this IncomingDatagramStream, maxResults uint64) witgo.Result[[]IncomingDatagram, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[[]IncomingDatagram, ErrorCode](ErrorCodeInvalidArgument)
	}

	if maxResults == 0 {
		return witgo.Ok[[]IncomingDatagram, ErrorCode]([]IncomingDatagram{})
	}

	datagrams := make([]IncomingDatagram, 0, maxResults)

	// 缓冲区，64k 是 UDP 数据报的理论最大值
	buf := make([]byte, 65535)

	for len(datagrams) < int(maxResults) {
		// 使用 ReadFromUDP 来接收数据和发送方地址
		n, remoteAddr, err := sock.Conn.ReadFromUDP(buf)
		if err != nil {
			// 如果是会阻塞的错误，说明没有更多数据可读，正常返回已接收的数据
			if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, os.ErrDeadlineExceeded) {
				break
			}
			return witgo.Err[[]IncomingDatagram, ErrorCode](mapOsError(err))
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		wasiAddr, addrErr := toIPSocketAddress(remoteAddr)
		if addrErr != nil {
			// 如果地址转换失败，跳过这个数据报
			continue
		}

		datagrams = append(datagrams, IncomingDatagram{
			Data:          data,
			RemoteAddress: wasiAddr,
		})
	}

	return witgo.Ok[[]IncomingDatagram, ErrorCode](datagrams)
}

func (i *udpImpl) Send(_ context.Context, this OutgoingDatagramStream, datagrams []OutgoingDatagram) witgo.Result[uint64, ErrorCode] {
	sock, ok := i.host.UDPSocketManager().Get(this)
	if !ok || sock.Conn == nil {
		return witgo.Err[uint64, ErrorCode](ErrorCodeInvalidArgument)
	}

	var sentCount uint64
	for _, dg := range datagrams {
		var remoteAddr net.Addr
		var err error

		if dg.RemoteAddress.Some != nil {
			remoteAddr, err = fromIPSocketAddress(*dg.RemoteAddress.Some)
			if err != nil {
				// 如果地址无效，并且我们已经发送了一些数据报，就此打住
				if sentCount > 0 {
					break
				}
				return witgo.Err[uint64, ErrorCode](ErrorCodeInvalidArgument)
			}
		}

		// 如果 remoteAddr 不为 nil，使用 WriteTo；否则使用 Write (用于 "connected" UDP socket)
		var n int
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			n, err = sock.Conn.WriteToUDP(dg.Data, udpAddr)
		} else {
			n, err = sock.Conn.Write(dg.Data)
		}

		if err != nil {
			// 如果发生错误，并且我们已经成功发送了一些数据报，那么根据规范，我们应该返回成功发送的数量
			if sentCount > 0 {
				break
			}
			return witgo.Err[uint64, ErrorCode](mapOsError(err))
		}
		if n != len(dg.Data) {
			// 数据未完全发送，这是一个异常情况
			if sentCount > 0 {
				break
			}
			return witgo.Err[uint64, ErrorCode](ErrorCodeUnknown)
		}
		sentCount++
	}

	return witgo.Ok[uint64, ErrorCode](sentCount)
}

func (i *udpImpl) CheckSend(_ context.Context, this OutgoingDatagramStream) witgo.Result[uint64, ErrorCode] {
	// 总是允许发送，简化实现
	return witgo.Ok[uint64, ErrorCode](1)
}

func (i *udpImpl) SubscribeIncoming(ctx context.Context, this IncomingDatagramStream) wasip2_io.Pollable {
	return i.Subscribe(ctx, this)
}
func (i *udpImpl) SubscribeOutgoing(ctx context.Context, this OutgoingDatagramStream) wasip2_io.Pollable {
	return i.Subscribe(ctx, this)
}
