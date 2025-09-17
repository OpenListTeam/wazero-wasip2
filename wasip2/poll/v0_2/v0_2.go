package v0_2

import (
	"context"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

type wasiPoll struct{}

func New() wasip2.Implementation {
	return &wasiPoll{}
}

func (i *wasiPoll) Name() string { return "wasi:io/poll" }
func (i *wasiPoll) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiPoll) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	handler := newPollImpl(h.PollManager())
	exporter := witgo.NewExporter(builder)
	exporter.Export("[resource-drop]pollable", handler.DropPollable)
	exporter.Export("[method]pollable.ready", handler.Ready)
	exporter.Export("[method]pollable.block", handler.Block)

	exporter.Export("poll", handler.Poll)
	return nil
}
