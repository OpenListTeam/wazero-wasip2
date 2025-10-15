package wasi_tls

import (
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	v0_2 "github.com/OpenListTeam/wazero-wasip2/wasip2/tls/v0_2"
)

// Module returns a configured wasi:tls module option.
func Module(version string) wasip2.ModuleOption {
	return func(h *wasip2.Host) {
		var typesImpl wasip2.Implementation

		switch version {
		case "0.2.0-draft":
			typesImpl = v0_2.NewTypes(h.TLSManager(), h.StreamManager(), h.ErrorManager())
		default:
			return
		}
		h.AddImplementation(typesImpl)
	}
}
