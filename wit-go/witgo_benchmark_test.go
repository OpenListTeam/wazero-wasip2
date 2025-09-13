package witgo

import (
	_ "embed"
)

// Benchmark lifting and lowering a complex record.
// func BenchmarkLiftLowerMyData(b *testing.B) {
// 	ctx := context.Background()
// 	r := wazero.NewRuntime(ctx)
// 	defer r.Close(ctx)
// 	wasi_snapshot_preview1.MustInstantiate(ctx, r)
// 	mod, _ := r.Instantiate(ctx, guestWasm)
// 	host, _ := NewHost(mod)

// 	// Prepare a sample Go struct
// 	goData := MyData{
// 		A: 42,
// 		B: "a moderately long string for testing purposes",
// 		C: make([]byte, 1024),
// 	}

// 	b.Run("Lift", func(b *testing.B) {
// 		b.ReportAllocs()
// 		// Reset timer to exclude setup time.
// 		b.ResetTimer()
// 		for i := 0; i < b.N; i++ {
// 			// In a real benchmark, you would also need to free the memory
// 			// to avoid exhausting it, or use a fresh memory for each run.
// 			_, err := host.Call(ctx, "process-data-noop", nil, goData) // Assume a no-op guest func
// 			if err != nil {
// 				b.Fatal(err)
// 			}
// 		}
// 	})

// 	b.Run("Lower", func(b *testing.B) {
// 		// First, call the guest once to get a pointer to some data.
// 		results, _ := mod.ExportedFunction("create-data").Call(ctx)
// 		ptr := uint32(results[0])

// 		b.ReportAllocs()
// 		b.ResetTimer()

// 		var result MyData
// 		for i := 0; i < b.N; i++ {
// 			// Lower the data from the same pointer repeatedly.
// 			err := host.lower(ctx, ptr, &result) // Assuming lower is exported for testing
// 			if err != nil {
// 				b.Fatal(err)
// 			}
// 		}
// 	})
// }
