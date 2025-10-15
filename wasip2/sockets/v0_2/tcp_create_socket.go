package v0_2

import (
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
)

type tcpCreateSocketImpl struct {
	host *wasip2.Host
}

func newTCPCreateSocketImpl(h *wasip2.Host) *tcpCreateSocketImpl {
	return &tcpCreateSocketImpl{host: h}
}
