package wasi_clocks

import (
	"github.com/foxxorcat/wazero-wasip2/wasip2"
	v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/clocks/v0_2"
)

// Module 返回一个配置好的 wasi:clocks 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var monotonicClockImpl, wallClockImpl wasip2.Implementation

		switch version {
		case "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5":
			monotonicClockImpl = v0_2.NewMonotonicClock()
			wallClockImpl = v0_2.NewWallClock()
		default:
			return
		}
		h.AddImplementation(monotonicClockImpl)
		h.AddImplementation(wallClockImpl)
	}
}
