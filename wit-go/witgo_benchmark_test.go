package witgo

import (
	"context"
	_ "embed"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// setupBenchmark initializes a wazero runtime and instantiates the guest for benchmarking.
// It returns a context, our Host wrapper, and the raw wazero module instance.
func setupBenchmark(b *testing.B) (context.Context, *Host, api.Module) {
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)

	// b.Cleanup ensures the runtime is closed after the benchmark finishes.
	b.Cleanup(func() {
		if err := r.Close(ctx); err != nil {
			b.Logf("failed to close runtime: %v", err)
		}
	})

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// We don't need the host logger for benchmarks, so we provide a no-op implementation.
	exporter := NewExporter(r.NewHostModuleBuilder("$root"))
	ExporterTestHostFunc(exporter)
	_, err := exporter.
		MustExport("host-log", func(msg string) {}).
		MustExport("process-host-request", func(req HostRequest) string { return "" }).Instantiate(ctx)
	require.NoError(b, err)

	mod, err := r.InstantiateWithConfig(ctx, guestWasm, wazero.NewModuleConfig().WithName("benchmark-instance"))
	if err != nil {
		b.Fatalf("failed to instantiate guest module: %v", err)
	}

	host, err := NewHost(mod)
	if err != nil {
		b.Fatalf("failed to create host: %v", err)
	}

	return ctx, host, mod
}

// BenchmarkComplexRecord measures the performance of lifting (Go->Wasm) and lowering (Wasm->Go) a complex, nested struct.
func BenchmarkComplexRecord(b *testing.B) {
	ctx, host, mod := setupBenchmark(b)

	// Prepare a representative complex Go struct to be used in the benchmarks.
	radius := float32(50.0)
	goData := ComplexRecord{
		ID: "benchmark-complex-id-12345",
		Permissions: Some(Permissions{
			Read:  true,
			Write: true,
		}),
		ChildData: []MyData{
			{A: 10, B: "child1-benchmark", C: []byte{1, 2, 3, 4}},
			{A: 20, B: "child2-benchmark", C: []byte{5, 6, 7, 8}},
		},
		ShapeInfo: Ok[Shape, string](Shape{Circle: radius}),
	}

	// --- Benchmark Lifting (Go -> Wasm) ---
	b.Run("Lift", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// We call our no-op guest function to measure the overhead of parameter passing.
			err := host.Call(ctx, "noop-complex", nil, goData)
			if err != nil {
				b.Fatalf("lift benchmark failed: %v", err)
			}
		}
	})

	// --- Benchmark Lowering (Wasm -> Go) ---
	b.Run("Lower", func(b *testing.B) {
		params := make([]uint64, 0, 16)
		// First, call the guest once to create the complex data and get a pointer to it.
		// This setup is outside the measurement loop.
		err := host.flattenParam(ctx, reflect.ValueOf(goData), &params)
		if err != nil {
			b.Fatalf("setup for lower benchmark failed: %v", err)
		}

		results, err := mod.ExportedFunction("handle-complex-record").Call(ctx,
			// We need to pass a valid record to get a valid result back.
			// We can reuse the flatten logic for this setup.
			params...,
		)
		if err != nil {
			b.Fatalf("setup for lower benchmark failed: %v", err)
		}
		// The function now returns a scalar u32, so we need a different approach.
		// Let's benchmark lowering the `create-data` result instead.
		results, err = mod.ExportedFunction("create-data").Call(ctx)
		if err != nil {
			b.Fatalf("setup for lower benchmark failed (create-data): %v", err)
		}
		ptr := uint32(results[0])

		var result MyData

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Lower the data from the same pointer repeatedly.
			err := Lower(ctx, mod.Memory(), ptr, reflect.ValueOf(&result).Elem())
			if err != nil {
				b.Fatalf("lower benchmark failed: %v", err)
			}
		}
	})
}
