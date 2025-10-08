package v0_2

import (
	manager_sockets "github.com/foxxorcat/wazero-wasip2/manager/sockets"
	io_v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
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

type IPAddressFamily = manager_sockets.IPAddressFamily

const (
	IPAddressFamilyIPV4 = manager_sockets.IPAddressFamilyIPV4
	IPAddressFamilyIPV6 = manager_sockets.IPAddressFamilyIPV6
)

// --- Records and Variants ---

type IPv4Address = manager_sockets.IPv4Address
type IPv6Address = manager_sockets.IPv6Address

type IPAddress = manager_sockets.IPAddress
type IPv4SocketAddress = manager_sockets.IPv4SocketAddress
type IPv6SocketAddress = manager_sockets.IPv6SocketAddress

type IPSocketAddress = manager_sockets.IPSocketAddress

// IncomingDatagram 代表一个接收到的 UDP 数据报。
type IncomingDatagram = manager_sockets.IncomingDatagram

// OutgoingDatagram 代表一个待发送的 UDP 数据报。
type OutgoingDatagram = manager_sockets.OutgoingDatagram
