package wasi_http

import (
	"wazero-wasip2/wasip2"
	v0_2 "wazero-wasip2/wasip2/http/v0_2"
)

// Module 返回一个配置好的 wasi:http 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var typesImpl, outgoingHandlerImpl wasip2.Implementation

		switch version {
		case "0.2.0", "0.2.1", "0.2.2":
			// 创建 types 和 outgoing-handler 的实现实例，
			// 并将 Host 中的管理器注入进去。
			typesImpl = v0_2.NewTypes(h.HTTPManager())
			outgoingHandlerImpl = v0_2.NewOutgoingHandler(h.HTTPManager())
		default:
			return
		}
		h.AddImplementation(typesImpl)
		h.AddImplementation(outgoingHandlerImpl)
	}
}
