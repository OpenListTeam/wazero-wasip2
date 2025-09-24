package v0_2

import (
	"context"
	manager_http "wazero-wasip2/internal/http"
	"wazero-wasip2/wasip2"

	"github.com/tetratelabs/wazero"
)

// --- wasi:http/incoming-handler@0.2.0 implementation ---

type incomingHandler struct {
	hm *manager_http.HTTPManager
}

func NewIncomingHandler(hm *manager_http.HTTPManager) wasip2.Implementation {
	return &incomingHandler{hm: hm}
}

func (i *incomingHandler) Name() string       { return "wasi:http/incoming-handler" }
func (i *incomingHandler) Versions() []string { return []string{"0.2.0", "0.2.1", "0.2.2"} }

func (i *incomingHandler) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	// The `handle` function is exported by the guest, so the host does not export it.
	// This interface is defined to satisfy the world definition, allowing the guest
	// to correctly export its implementation.
	return nil
}
