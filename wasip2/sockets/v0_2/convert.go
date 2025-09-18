package v0_2

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"net"
	"syscall"
	"wazero-wasip2/internal/sockets"
)

// fromIPAddressFamily 将 WIT 的 IPAddressFamily 转换为内部的通用类型。
func fromIPAddressFamily(family IPAddressFamily) (sockets.IPAddressFamily, error) {
	switch family {
	case IPAddressFamilyIPV4:
		return sockets.IPAddressFamilyIPV4, nil
	case IPAddressFamilyIPV6:
		return sockets.IPAddressFamilyIPV6, nil
	default:
		return 0, errors.New("invalid ip-address-family")
	}
}

// toIPAddressFamily 将内部的通用类型转换为 WIT 的 IPAddressFamily。
func toIPAddressFamily(family sockets.IPAddressFamily) (IPAddressFamily, error) {
	switch family {
	case sockets.IPAddressFamilyIPV4:
		return IPAddressFamilyIPV4, nil
	case sockets.IPAddressFamilyIPV6:
		return IPAddressFamilyIPV6, nil
	default:
		return 0, errors.New("invalid ip-address-family")
	}
}

// fromIPSocketAddressToTCPAddr 将 WIT 的 IPSocketAddress 转换为 Go 的 *net.TCPAddr。
func fromIPSocketAddressToTCPAddr(addr IPSocketAddress) (*net.TCPAddr, error) {
	if addr.IPV4 != nil {
		ip := net.IP(addr.IPV4.Address[:])
		return &net.TCPAddr{IP: ip, Port: int(addr.IPV4.Port)}, nil
	}
	if addr.IPV6 != nil {
		ip := make(net.IP, 16)
		for i, part := range addr.IPV6.Address {
			binary.BigEndian.PutUint16(ip[i*2:], part)
		}
		// ZoneId 的转换依赖于 net.LookupAddr 或其他更复杂的逻辑，此处简化。
		return &net.TCPAddr{IP: ip, Port: int(addr.IPV6.Port), Zone: ""}, nil
	}
	return nil, errors.New("invalid ip-socket-address")
}

// fromIPSocketAddressToUDPAddr 将 WIT 的 IPSocketAddress 转换为 Go 的 *net.UDPAddr。
func fromIPSocketAddressToUDPAddr(addr IPSocketAddress) (*net.UDPAddr, error) {
	if addr.IPV4 != nil {
		ip := net.IP(addr.IPV4.Address[:])
		return &net.UDPAddr{IP: ip, Port: int(addr.IPV4.Port)}, nil
	}
	if addr.IPV6 != nil {
		ip := make(net.IP, 16)
		for i, part := range addr.IPV6.Address {
			binary.BigEndian.PutUint16(ip[i*2:], part)
		}
		return &net.UDPAddr{IP: ip, Port: int(addr.IPV6.Port), Zone: ""}, nil
	}
	return nil, errors.New("invalid ip-socket-address")
}

// mapDnsError 将 Go 的 net.DNSError 映射到 wasi:sockets 的 ErrorCode。
func mapDnsError(err error) ErrorCode {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTemporary {
			return ErrorCodeTemporaryResolverFailure
		}
		if dnsErr.IsNotFound {
			return ErrorCodeNameUnresolvable
		}
		// 如果不是临时或未找到，则认为是永久性故障
		return ErrorCodePermanentResolverFailure
	}
	// 对于其他类型的网络错误，返回一个通用的不可解析错误
	return ErrorCodeNameUnresolvable
}

// mapOsError 将 Go 的 os/syscall 网络错误映射到 wasi:sockets 的 ErrorCode。
func mapOsError(err error) ErrorCode {
	if err == nil {
		return 0 // Not an error
	}
	if errors.Is(err, fs.ErrPermission) {
		return ErrorCodeAccessDenied
	}
	if errors.Is(err, fs.ErrInvalid) {
		return ErrorCodeInvalidArgument
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		err = opErr.Err // 深入到根本的 syscall 错误
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES, syscall.EPERM:
			return ErrorCodeAccessDenied
		case syscall.EADDRINUSE:
			return ErrorCodeAddressInUse
		case syscall.EADDRNOTAVAIL:
			return ErrorCodeAddressNotBindable
		case syscall.EAFNOSUPPORT:
			return ErrorCodeNotSupported
		case syscall.EALREADY:
			return ErrorCodeConcurrencyConflict
		case syscall.ECONNABORTED:
			return ErrorCodeConnectionAborted
		case syscall.ECONNREFUSED:
			return ErrorCodeConnectionRefused
		case syscall.ECONNRESET:
			return ErrorCodeConnectionReset
		case syscall.EINPROGRESS:
			// WASI 0.2 中通常由 would-block 表示
			return ErrorCodeConcurrencyConflict
		case syscall.EINVAL:
			return ErrorCodeInvalidArgument
		case syscall.EISCONN:
			return ErrorCodeInvalidState
		case syscall.ENETUNREACH:
			return ErrorCodeRemoteUnreachable
		case syscall.ENFILE, syscall.EMFILE:
			return ErrorCodeNewSocketLimit
		case syscall.ENOTCONN:
			return ErrorCodeInvalidState
		case syscall.EOPNOTSUPP:
			return ErrorCodeNotSupported
		case syscall.ETIMEDOUT:
			return ErrorCodeTimeout
		case syscall.EWOULDBLOCK:
			return ErrorCodeWouldBlock
		}
	}

	return ErrorCodeUnknown
}

// toIPSocketAddress 将 Go 的 net.Addr 转换为 WIT 的 IPSocketAddress。
func toIPSocketAddress(addr net.Addr) (IPSocketAddress, error) {
	switch tcpAddr := addr.(type) {
	case *net.TCPAddr:

		if ipv4 := tcpAddr.IP.To4(); ipv4 != nil {
			var wasiAddr IPv4Address
			copy(wasiAddr[:], ipv4)
			return IPSocketAddress{
				IPV4: &IPv4SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
				},
			}, nil
		}

		if ipv6 := tcpAddr.IP.To16(); ipv6 != nil {
			var wasiAddr IPv6Address
			for i := 0; i < 8; i++ {
				wasiAddr[i] = binary.BigEndian.Uint16(ipv6[i*2:])
			}
			return IPSocketAddress{
				IPV6: &IPv6SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
					// FlowInfo and ScopeID require more complex logic to extract
				},
			}, nil
		}
	case *net.UDPAddr:
		if ipv4 := tcpAddr.IP.To4(); ipv4 != nil {
			var wasiAddr IPv4Address
			copy(wasiAddr[:], ipv4)
			return IPSocketAddress{
				IPV4: &IPv4SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
				},
			}, nil
		}

		if ipv6 := tcpAddr.IP.To16(); ipv6 != nil {
			var wasiAddr IPv6Address
			for i := 0; i < 8; i++ {
				wasiAddr[i] = binary.BigEndian.Uint16(ipv6[i*2:])
			}
			return IPSocketAddress{
				IPV6: &IPv6SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
					// FlowInfo and ScopeID require more complex logic to extract
				},
			}, nil
		}
	default:
	}
	return IPSocketAddress{}, errors.New("address is not TCPAddr")
}

func fromIPSocketAddressToSockaddr(addr IPSocketAddress) (syscall.Sockaddr, error) {
	if addr.IPV4 != nil {
		return &syscall.SockaddrInet4{
			Port: int(addr.IPV4.Port),
			Addr: addr.IPV4.Address,
		}, nil
	}
	if addr.IPV6 != nil {
		var ip [16]byte
		for i, part := range addr.IPV6.Address {
			binary.BigEndian.PutUint16(ip[i*2:], part)
		}
		return &syscall.SockaddrInet6{
			Port:   int(addr.IPV6.Port),
			ZoneId: addr.IPV6.ScopeID,
			Addr:   ip,
		}, nil
	}
	return nil, errors.New("invalid ip-socket-address")
}

// toIPAddress 将 Go 的 net.IP 转换为 WIT 的 IPAddress。
func toIPAddress(ip net.IP) (IPAddress, error) {
	if ipv4 := ip.To4(); ipv4 != nil {
		var wasiAddr IPv4Address
		copy(wasiAddr[:], ipv4)
		return IPAddress{IPV4: &wasiAddr}, nil
	}
	if ipv6 := ip.To16(); ipv6 != nil {
		var wasiAddr IPv6Address
		for i := 0; i < 8; i++ {
			wasiAddr[i] = binary.BigEndian.Uint16(ipv6[i*2:])
		}
		return IPAddress{IPV6: &wasiAddr}, nil
	}
	return IPAddress{}, errors.New("unsupported IP address format")
}
