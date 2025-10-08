package v0_2

import (
	"github.com/foxxorcat/wazero-wasip2/wasip2"
)

type udpCreateSocketImpl struct {
	host *wasip2.Host
}

func newUDPCreateSocketImpl(h *wasip2.Host) *udpCreateSocketImpl {
	return &udpCreateSocketImpl{host: h}
}
