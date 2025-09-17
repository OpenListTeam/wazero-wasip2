package wasi_stream

import (
	"wazero-wasip2/wasip2"
	"wazero-wasip2/wasip2/stream/v0_2"
)

// Module 返回一个配置好的 wasi:io 模块选项。
// 用户通过调用 wasip2.NewHost(wasi_io.Module("0.2.0")) 来启用此模块。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var impl wasip2.Implementation

		switch version {
		// 所有 0.2.x 版本都共享同一套实现
		case "0.2", "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			impl = v0_2.New()
		default:
			// 如果用户请求一个不支持的版本，我们什么都不做。
			return
		}
		h.AddImplementation(impl)
	}
}
