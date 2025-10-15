package wasi_io

import (
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	v0_2 "github.com/OpenListTeam/wazero-wasip2/wasip2/io/v0_2"
)

// Module 返回一个配置好的 wasi:io 模块选项。
// 用户通过调用 wasip2.NewHost(wasi_io.Module("0.2.0")) 来启用此模块。
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var errorImpl, pollImpl, streamsImpl wasip2.Implementation

		switch version {
		case "0.2", "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7":
			errorImpl = v0_2.NewError()
			pollImpl = v0_2.NewPoll()
			streamsImpl = v0_2.NewStreams()
		default:
			return
		}
		h.AddImplementation(errorImpl)
		h.AddImplementation(pollImpl)
		h.AddImplementation(streamsImpl)
	}
}
