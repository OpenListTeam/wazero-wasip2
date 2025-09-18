//go:build unix && darwin

package v0_2

import (
	"context"
	witgo "wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *tcpImpl) KeepAliveIdleTime(ctx context.Context, this TCPSocket) witgo.Result[uint64, ErrorCode] {
	return getsockoptInt[uint64](i, this, unix.IPPROTO_TCP, unix.TCP_KEEPALIVE)
}

func (i *tcpImpl) SetKeepAliveIdleTime(ctx context.Context, this TCPSocket, value uint64) witgo.Result[witgo.Unit, ErrorCode] {
	return setsockoptInt(i, this, unix.IPPROTO_TCP, unix.TCP_KEEPALIVE, int(value))
}
