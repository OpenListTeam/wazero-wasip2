package v0_2

import (
	"context"

	"github.com/foxxorcat/wazero-wasip2/wasip2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:random/random@0.2.7 implementation ---

type wasiRandom struct{}

func NewRandom() wasip2.Implementation {
	return &wasiRandom{}
}

func (i *wasiRandom) Name() string { return "wasi:random/random" }
func (i *wasiRandom) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiRandom) Instantiate(_ context.Context, _ *wasip2.Host, builder wazero.HostModuleBuilder) error {
	exporter := witgo.NewExporter(builder)
	handler := newRandomImpl()
	exporter.Export("get-random-bytes", handler.GetRandomBytes)
	exporter.Export("get-random-u64", handler.GetRandomU64)
	return nil
}

// --- wasi:random/insecure@0.2.7 implementation ---

type wasiInsecure struct{}

func NewInsecure() wasip2.Implementation {
	return &wasiInsecure{}
}

func (i *wasiInsecure) Name() string { return "wasi:random/insecure" }
func (i *wasiInsecure) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiInsecure) Instantiate(_ context.Context, _ *wasip2.Host, builder wazero.HostModuleBuilder) error {
	exporter := witgo.NewExporter(builder)
	handler := newInsecureImpl()
	exporter.Export("get-insecure-random-bytes", handler.GetInsecureRandomBytes)
	exporter.Export("get-insecure-random-u64", handler.GetInsecureRandomU64)
	return nil
}

// --- wasi:random/insecure-seed@0.2.7 implementation ---

type wasiInsecureSeed struct{}

func NewInsecureSeed() wasip2.Implementation {
	return &wasiInsecureSeed{}
}

func (i *wasiInsecureSeed) Name() string { return "wasi:random/insecure-seed" }
func (i *wasiInsecureSeed) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiInsecureSeed) Instantiate(_ context.Context, _ *wasip2.Host, builder wazero.HostModuleBuilder) error {
	exporter := witgo.NewExporter(builder)
	handler := newInsecureSeedImpl()
	exporter.Export("insecure-seed", handler.InsecureSeed)
	return nil
}
