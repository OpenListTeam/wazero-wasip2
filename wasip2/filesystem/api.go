package wasi_filesystem

import (
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	v0_2 "github.com/OpenListTeam/wazero-wasip2/wasip2/filesystem/v0_2"
)

// Module 返回一个配置好的 wasi:filesystem 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var typesImpl, preopensImpl wasip2.Implementation

		switch version {
		case "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			typesImpl = v0_2.NewTypes()
			preopensImpl = v0_2.NewPreopens()
		default:
			return
		}
		h.AddImplementation(typesImpl)
		h.AddImplementation(preopensImpl)
	}
}
