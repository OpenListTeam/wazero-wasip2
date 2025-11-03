package sockets

import (
	"errors"
	"net"
	"sync"

	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

// IPAddressFamily 是一个版本无关的 IP 地址族枚举。
type IPAddressFamily uint8

const (
	IPAddressFamilyIPV4 IPAddressFamily = iota
	IPAddressFamilyIPV6
)

type IPv4Address = [4]byte
type IPv6Address = [8]uint16

type IPAddress struct {
	IPV4 *IPv4Address `wit:"case(0)"`
	IPV6 *IPv6Address `wit:"case(1)"`
}

type IPv4SocketAddress struct {
	Port    uint16
	Address IPv4Address
}

type IPv6SocketAddress struct {
	Port     uint16
	FlowInfo uint32
	Address  IPv6Address
	ScopeID  uint32
}

type IPSocketAddress struct {
	IPV4 *IPv4SocketAddress `wit:"case(0)"`
	IPV6 *IPv6SocketAddress `wit:"case(1)"`
}

// IncomingDatagram 代表一个接收到的 UDP 数据报。
type IncomingDatagram struct {
	Data          []byte
	RemoteAddress IPSocketAddress
}

// OutgoingDatagram 代表一个待发送的 UDP 数据报。
type OutgoingDatagram struct {
	Data          []byte
	RemoteAddress witgo.Option[IPSocketAddress]
}

// Network represents the capability to access the network.
// In this implementation, it's a placeholder struct.
type Network struct{}

// ConnectResult 用于在 goroutine 之间传递异步连接的结果。
type ConnectResult struct {
	Conn *net.TCPConn
	Err  error
}

// TCPSocket 代表一个 TCP 套接字资源。
type TCPSocket struct {
	Fd       int
	Listener *net.TCPListener
	Conn     *net.TCPConn
	Family   IPAddressFamily
	State    TCPState

	// ConnectResult 用于异步 connect 操作。
	// 当 start-connect 被调用时，一个 goroutine 会开始连接，
	// 并将结果（一个 ConnectResult）发送到这个 channel。
	// Channel is managed by background goroutine and closed when done.
	ConnectResult chan ConnectResult

	// ConnectCancel cancels the background connect goroutine.
	// Must be called before ConnectResult becomes invalid.
	ConnectCancel func()

	// Ensure Done channel is only closed once
	closeOnce sync.Once
}

// Close releases all resources associated with the TCPSocket
func (s *TCPSocket) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.Fd != 0 {
			CloseFd(s.Fd)
		}
		if s.Listener != nil {
			err = s.Listener.Close()
		}
		if s.Conn != nil {
			if closeErr := s.Conn.Close(); closeErr != nil && err == nil {
				err = errors.Join(err, closeErr)
			}
		}

		if s.ConnectResult != nil {
			close(s.ConnectResult)
			s.ConnectResult = nil
		}
		if s.ConnectCancel != nil {
			s.ConnectCancel()
			s.ConnectCancel = nil
		}
		s.State = TCPStateClosed
	})
	return err
}

// TCPState represents the state of a TCP socket as defined in the WIT world.
type TCPState uint8

const (
	TCPStateUnbound TCPState = iota
	TCPStateBound
	TCPStateListening
	TCPStateConnecting
	TCPStateConnected
	TCPStateClosed
)

// UDPSocket represents a UDP socket resource.
type UDPSocket struct {
	Fd int
	// The Go standard library UDP connection.
	Conn *net.UDPConn
	// The address family of the socket.
	Family IPAddressFamily

	// 新增字段
	Reader *AsyncUDPReader
	Writer *AsyncUDPWriter
}

// Close releases all resources associated with the UDPSocket
func (s *UDPSocket) Close() error {
	if s.Fd != 0 {
		CloseFd(s.Fd)
	}
	if s.Reader != nil {
		s.Reader.Close()
	}
	if s.Writer != nil {
		s.Writer.Close()
	}
	if s.Conn != nil {
		return s.Conn.Close()
	}
	return nil
}

// ResolveAddressStreamState 保存了域名解析操作的状态。
type ResolveAddressStreamState struct {
	// 存储解析出的 IP 地址列表。
	Addresses []net.IP
	// 当前已返回给 Guest 的地址索引。
	Index int
	// 在异步解析过程中可能发生的错误。
	Error error
	// 一个 channel，当后台解析任务完成时，它会被关闭。
	Done chan struct{}
	// Ensure Done channel is only closed once
	closeOnce sync.Once
}

// CloseDone safely closes the Done channel using sync.Once
func (s *ResolveAddressStreamState) CloseDone() {
	s.closeOnce.Do(func() {
		if s.Done != nil {
			close(s.Done)
		}
	})
}

// --- Resource Managers ---

type NetworkManager = witgo.ResourceManager[*Network]
type TCPSocketManager = witgo.ResourceManager[*TCPSocket]
type UDPSocketManager = witgo.ResourceManager[*UDPSocket]
type ResolveAddressStreamManager = witgo.ResourceManager[*ResolveAddressStreamState]

func NewNetworkManager() *NetworkManager {
	return witgo.NewResourceManager[*Network](nil)
}
func NewTCPSocketManager() *TCPSocketManager {
	return witgo.NewResourceManager[*TCPSocket](func(socket *TCPSocket) {
		if socket != nil {
			socket.Close()
		}
	})
}
func NewUDPSocketManager() *UDPSocketManager {
	return witgo.NewResourceManager[*UDPSocket](func(socket *UDPSocket) {
		if socket != nil {
			socket.Close()
		}
	})
}
func NewResolveAddressStreamManager() *ResolveAddressStreamManager {
	return witgo.NewResourceManager[*ResolveAddressStreamState](func(state *ResolveAddressStreamState) {
		if state != nil {
			state.CloseDone()
		}
	})
}
