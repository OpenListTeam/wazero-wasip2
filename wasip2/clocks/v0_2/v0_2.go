package v0_2

import (
	"context"

	"github.com/foxxorcat/wazero-wasip2/wasip2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:clocks/monotonic-clock@0.2.5 implementation ---

type wasiMonotonicClock struct{}

func NewMonotonicClock() wasip2.Implementation {
	return &wasiMonotonicClock{}
}

func (i *wasiMonotonicClock) Name() string { return "wasi:clocks/monotonic-clock" }
func (i *wasiMonotonicClock) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiMonotonicClock) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newMonotonicClockImpl(h.PollManager())
	exporter := witgo.NewExporter(b)
	exporter.Export("now", handler.Now)
	exporter.Export("resolution", handler.Resolution)
	exporter.Export("subscribe-instant", handler.SubscribeInstant)
	exporter.Export("subscribe-duration", handler.SubscribeDuration)
	return nil
}

// --- wasi:clocks/wall-clock@0.2.5 implementation ---

type wasiWallClock struct{}

func NewWallClock() wasip2.Implementation {
	return &wasiWallClock{}
}

func (i *wasiWallClock) Name() string { return "wasi:clocks/wall-clock" }
func (i *wasiWallClock) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5"}
}

func (i *wasiWallClock) Instantiate(_ context.Context, _ *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newWallClockImpl()
	exporter := witgo.NewExporter(b)
	exporter.Export("now", handler.Now)
	exporter.Export("resolution", handler.Resolution)
	return nil
}
