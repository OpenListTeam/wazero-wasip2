package wasi_poll

import (
	"wazero-wasip2/wasip2"
	v0_2 "wazero-wasip2/wasip2/poll/v0_2"
)

// Module 返回一个配置好的 wasi:io/poll 模块选项。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var impl wasip2.Implementation
		switch version {
		case "0.2", "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			impl = v0_2.New()
		default:
			return
		}
		h.AddImplementation(impl)
	}
}
