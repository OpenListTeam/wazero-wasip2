package wasi_sockets

import (
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	v0_2 "github.com/OpenListTeam/wazero-wasip2/wasip2/sockets/v0_2"
)

// Module 返回一个配置好的 wasi:sockets 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var networkImpl, instanceNetworkImpl, tcpImpl, tcpCreateSocketImpl, udpImpl, udpCreateSocketImpl, ipNameLookupImpl wasip2.Implementation

		switch version {
		case "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			networkImpl = v0_2.NewNetwork()
			instanceNetworkImpl = v0_2.NewInstanceNetwork()
			tcpImpl = v0_2.NewTCP()
			tcpCreateSocketImpl = v0_2.NewTCPCreateSocket()
			udpImpl = v0_2.NewUDP()
			udpCreateSocketImpl = v0_2.NewUDPCreateSocket()
			ipNameLookupImpl = v0_2.NewIPNameLookup()
		default:
			return
		}
		h.AddImplementation(networkImpl)
		h.AddImplementation(instanceNetworkImpl)
		h.AddImplementation(tcpImpl)
		h.AddImplementation(tcpCreateSocketImpl)
		h.AddImplementation(udpImpl)
		h.AddImplementation(udpCreateSocketImpl)
		h.AddImplementation(ipNameLookupImpl)
	}
}
