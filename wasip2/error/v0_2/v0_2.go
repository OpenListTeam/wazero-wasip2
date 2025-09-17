package v0_2

import (
	"context"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

type wasiError struct{}

func New() wasip2.Implementation {
	return &wasiError{}
}

func (i *wasiError) Name() string { return "wasi:io/error" }
func (i *wasiError) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiError) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	handler := newErrorImpl(h.ErrorManager())
	exporter := witgo.NewExporter(builder)

	exporter.Export("[resource-drop]error", handler.DropError)
	exporter.Export("[method]error.to-debug-string", handler.ToDebugString)
	return nil
}
