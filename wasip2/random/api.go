package wasi_random

import (
	"github.com/foxxorcat/wazero-wasip2/wasip2"
	v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/random/v0_2"
)

// Module 返回一个配置好的 wasi:random 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var randomImpl, insecureImpl, insecureSeedImpl wasip2.Implementation

		switch version {
		case "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			randomImpl = v0_2.NewRandom()
			insecureImpl = v0_2.NewInsecure()
			insecureSeedImpl = v0_2.NewInsecureSeed()
		default:
			return
		}
		h.AddImplementation(randomImpl)
		h.AddImplementation(insecureImpl)
		h.AddImplementation(insecureSeedImpl)
	}
}
