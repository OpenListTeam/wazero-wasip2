package v0_2

import (
	io_v0_2 "wazero-wasip2/wasip2/io/v0_2"
	witgo "wazero-wasip2/wit-go"
)

// --- Imported Types ---
type WasiError = io_v0_2.Error

// --- Base Types ---
type Network = uint32
type TCPSocket = uint32
type UDPSocket = uint32
type IncomingDatagramStream = uint32
type OutgoingDatagramStream = uint32
type ResolveAddressStream = uint32

// --- Enums and Flags ---

type ErrorCode uint8

const (
	ErrorCodeUnknown ErrorCode = iota
	ErrorCodeAccessDenied
	ErrorCodeNotSupported
	ErrorCodeInvalidArgument
	ErrorCodeOutOfMemory
	ErrorCodeTimeout
	ErrorCodeConcurrencyConflict
	ErrorCodeNotInProgress
	ErrorCodeWouldBlock
	ErrorCodeInvalidState
	ErrorCodeNewSocketLimit
	ErrorCodeAddressNotBindable
	ErrorCodeAddressInUse
	ErrorCodeRemoteUnreachable
	ErrorCodeConnectionRefused
	ErrorCodeConnectionReset
	ErrorCodeConnectionAborted
	ErrorCodeDatagramTooLarge
	ErrorCodeNameUnresolvable
	ErrorCodeTemporaryResolverFailure
	ErrorCodePermanentResolverFailure
)

// ShutdownType 定义了 TCP socket 的关闭方式。
type ShutdownType uint8

const (
	ShutdownTypeReceive ShutdownType = iota // 关闭接收方向
	ShutdownTypeSend                        // 关闭发送方向
	ShutdownTypeBoth                        // 关闭双向
)

type IPAddressFamily uint8

const (
	IPAddressFamilyIPV4 IPAddressFamily = iota
	IPAddressFamilyIPV6
)

// --- Records and Variants ---

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
