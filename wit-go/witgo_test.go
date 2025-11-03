package witgo

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed guest.wasm
var guestWasm []byte

// MyData is the Go equivalent of the `my-data` record in WIT.
// It must match the structure defined in test.wit for reflection to work.
type MyData struct {
	A uint32
	B string
	C []byte
}

// Corresponds to the `color` enum in WIT.
type Color uint8

const (
	ColorRed Color = iota
	ColorGreen
	ColorBlue
)

// Corresponds to the `shape` variant in WIT.
// We use struct tags to map fields to variant cases.
type Shape struct {
	Circle    float32   `wit:"case(0)"`
	Rectangle [2]uint32 `wit:"case(1)"`
}

type Permissions struct {
	Read    bool
	Write   bool
	Execute bool
}

func (Permissions) IsFlags() {}

type ComplexRecord struct {
	ID          string
	Permissions Option[Permissions]
	ChildData   []MyData // This is the key: a list of records
	ShapeInfo   Result[Shape, string]
}

// Corresponds to the `tuple<u32, u8, string>` in WIT.
// We use a struct because Go arrays are homogeneous.
type HeteroTuple struct {
	F0 uint32
	F1 uint8
	F2 string
}

// Corresponds to the `host-request` record in WIT.
type HostRequest struct {
	ID     string
	Data   []MyData
	Config Option[Permissions]
}

func ExporterTestHostFunc(exporter *Exporter) {
	// --- Export plain scalar identity functions ---
	exporter.MustExport("host-test-u8", func(v uint8) uint8 { return v })
	exporter.MustExport("host-test-s8", func(v int8) int8 { return v })
	exporter.MustExport("host-test-u16", func(v uint16) uint16 { return v })
	exporter.MustExport("host-test-s16", func(v int16) int16 { return v })
	exporter.MustExport("host-test-u32", func(v uint32) uint32 { return v })
	exporter.MustExport("host-test-s32", func(v int32) int32 { return v })
	exporter.MustExport("host-test-u64", func(v uint64) uint64 { return v })
	exporter.MustExport("host-test-s64", func(v int64) int64 { return v })
	exporter.MustExport("host-test-float32", func(v float32) float32 { return v })
	exporter.MustExport("host-test-float64", func(v float64) float64 { return v })
	exporter.MustExport("host-test-bool", func(v bool) bool { return v })

	// --- Export Option<T> identity functions for all scalar types ---
	exporter.MustExport("host-test-option-u8", func(v Option[uint8]) Option[uint8] { return v })
	exporter.MustExport("host-test-option-s8", func(v Option[int8]) Option[int8] { return v })
	exporter.MustExport("host-test-option-u16", func(v Option[uint16]) Option[uint16] { return v })
	exporter.MustExport("host-test-option-s16", func(v Option[int16]) Option[int16] { return v })
	exporter.MustExport("host-test-option-u32", func(v Option[uint32]) Option[uint32] { return v })
	exporter.MustExport("host-test-option-s32", func(v Option[int32]) Option[int32] { return v })
	exporter.MustExport("host-test-option-u64", func(v Option[uint64]) Option[uint64] { return v })
	exporter.MustExport("host-test-option-s64", func(v Option[int64]) Option[int64] { return v })
	exporter.MustExport("host-test-option-float32", func(v Option[float32]) Option[float32] { return v })
	exporter.MustExport("host-test-option-float64", func(v Option[float64]) Option[float64] { return v })
	exporter.MustExport("host-test-option-bool", func(v Option[bool]) Option[bool] { return v })
}

func TestWitGo(t *testing.T) {
	ctx := context.Background()

	// 1. Create a wazero runtime.
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// 2. Instantiate WASI, which is required for wasm32-wasi modules.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// 3. Create a host module to provide imported functions to the guest.
	var hostLogBuffer string

	// 1. Create an Exporter that wraps the wazero builder.
	exporter := NewExporter(r.NewHostModuleBuilder("$root"))
	ExporterTestHostFunc(exporter)

	// 2. Use the chainable, "Must" variant to export functions.
	_, err := exporter.
		MustExport("host-log", func(ctx context.Context, msg string) {
			hostLogBuffer = msg
			fmt.Printf("[HOST LOG]: %s\n", hostLogBuffer)
		}).
		MustExport("invert-bytes", func(data []byte) []byte {
			reversed := make([]byte, len(data))
			for i, b := range data {
				reversed[len(data)-1-i] = b
			}
			return reversed
		}).
		MustExport("process-host-request", func(req HostRequest) string {
			// Build a summary string based on the complex input.
			summary := fmt.Sprintf("Received request %s with %d data records.", req.ID, len(req.Data))
			if req.Config.Some != nil {
				if req.Config.Some.Write {
					summary += " Write permission is enabled."
				}
			} else {
				summary += " No config provided."
			}
			return summary
		}).
		Instantiate(ctx)
	require.NoError(t, err)

	// 4. Instantiate the guest module.
	mod, err := r.InstantiateWithConfig(ctx, guestWasm, wazero.NewModuleConfig().WithName("test-instance"))
	require.NoError(t, err)

	// 5. Create our high-level Host wrapper. This is the main API of wit-go.
	host, err := NewHost(mod)
	require.NoError(t, err)

	// --- Test Suite ---

	t.Run("Host to Guest: Call with string parameter", func(t *testing.T) {
		hostLogBuffer = ""
		inputString := "Hello from Go Host!"

		// Call the guest function. All ABI complexity is hidden by host.Call.
		// `resultPtr` is nil because the WIT function returns nothing.
		err := host.Call(ctx, "process-string", nil, inputString)
		require.NoError(t, err)

		// Assert that the guest correctly received the string and called the host logger.
		assert.Equal(t, "Guest received string: 'Hello from Go Host!'", hostLogBuffer)
	})

	t.Run("Guest to Host: Call with string return value", func(t *testing.T) {
		var result string // The Go variable to hold the result.
		inputString := "How are you?"

		// `host.Call` will handle the `retptr` logic and decoding automatically.
		err := host.Call(ctx, "roundtrip-string", &result, inputString)
		require.NoError(t, err)

		assert.Equal(t, "Guest says: How are you?", result)
	})

	t.Run("Guest to Host: Call with record return value", func(t *testing.T) {
		var result MyData // The Go struct to hold the complex result.

		// Call the guest function that creates and returns a record.
		err := host.Call(ctx, "create-data", &result)
		require.NoError(t, err)

		// Assert that the struct was correctly decoded from guest memory.
		expected := MyData{
			A: 123,
			B: "hello from guest",
			C: []byte{10, 20, 30},
		}
		assert.Equal(t, expected.A, result.A)
		assert.Equal(t, expected.B, result.B)
		assert.Equal(t, expected.C, result.C)
	})

	t.Run("Roundtrip Option: Some value", func(t *testing.T) {
		var result Option[uint32]
		input := Some("this is a test")
		err := host.Call(ctx, "handle-option", &result, input)
		require.NoError(t, err)

		require.True(t, result.Some != nil)
		assert.Equal(t, uint32(len("this is a test")), *result.Some)
	})

	t.Run("Roundtrip Option: None value", func(t *testing.T) {
		var result Option[uint32]
		input := None[string]()
		err := host.Call(ctx, "handle-option", &result, input)
		require.NoError(t, err)
		assert.True(t, result.None != nil)
	})

	t.Run("Roundtrip Result: Ok value", func(t *testing.T) {
		var result Result[uint32, Color]
		input := Ok[string, Color]("success")
		err := host.Call(ctx, "handle-result", &result, input)
		require.NoError(t, err)

		require.True(t, result.Ok != nil)
		assert.Equal(t, uint32(len("success")), *result.Ok)
	})

	t.Run("Roundtrip Result: Err value", func(t *testing.T) {
		var result Result[uint32, Color]
		input := Err[string, Color](ColorBlue)
		err := host.Call(ctx, "handle-result", &result, input)
		require.NoError(t, err)

		require.True(t, result.Err != nil)
		assert.Equal(t, ColorBlue, *result.Err)
	})

	t.Run("Host to Guest: Variant", func(t *testing.T) {
		var result string
		radius := float32(10.5)
		input := Shape{Circle: radius} // Pass a circle variant

		err := host.Call(ctx, "handle-variant", &result, input)
		require.NoError(t, err)
		assert.Equal(t, "Circle with radius 10.5", result)
	})

	t.Run("Handle Flags", func(t *testing.T) {
		var result []string
		// Pass a flags struct as a parameter.
		input := Permissions{Read: true, Execute: true}
		err := host.Call(ctx, "handle-permissions", &result, input)
		require.NoError(t, err)

		// Check that the guest correctly interpreted the bitmask.
		assert.Equal(t, []string{"read", "execute"}, result)
	})

	t.Run("Handle List of Records", func(t *testing.T) {
		var result []string
		input := []MyData{
			{A: 1, B: "Alice", C: []byte{1, 0}},
			{A: 2, B: "Bob", C: []byte{2, 0}},
		}
		err := host.Call(ctx, "process-users", &result, input)
		require.NoError(t, err)

		// Check that the guest correctly processed the list and returned the names.
		assert.Equal(t, []string{"Alice", "Bob"}, result)
	})

	t.Run("Handle Complex Nested Record", func(t *testing.T) {
		var result uint32
		radius := float32(50.0)
		input := ComplexRecord{
			ID: "complex-id-123",
			Permissions: Some(Permissions{
				Read:  true,
				Write: true,
			}),
			ChildData: []MyData{
				{A: 10, B: "child1", C: []byte{1}},
				{A: 20, B: "child2", C: []byte{2}},
			},
			ShapeInfo: Ok[Shape, string](Shape{Circle: radius}),
		}

		err := host.Call(ctx, "handle-complex-record", &result, input)
		require.NoError(t, err)

		// Checksum calculated by hand from the input data based on guest logic:
		// len("complex-id-123") -> 14
		// Permissions has Write -> +100
		// len(ChildData) * 1000 -> 2 * 1000 = 2000
		// Child checksum -> 10 + 20 = 30
		// ShapeInfo is Circle -> +50
		// Total: 14 + 100 + 2000 + 30 + 50 = 2194
		assert.Equal(t, uint32(2194), result)
	})

	t.Run("Handle Heterogeneous Tuple", func(t *testing.T) {
		var result string
		input := HeteroTuple{
			F0: 99,
			F1: 255,
			F2: "tuple test",
		}

		// Our existing struct handling logic should flatten this correctly.
		err := host.Call(ctx, "handle-hetero-tuple", &result, input)
		require.NoError(t, err)

		assert.Equal(t, "Got tuple: (99, 255, 'tuple test')", result)
	})

	t.Run("Call Complex Host Function", func(t *testing.T) {
		var result string
		input := HostRequest{
			ID: "request-ABC",
			Data: []MyData{
				{A: 1, B: "first", C: []byte{1}},
				{A: 2, B: "second", C: []byte{2}},
			},
			Config: Some(Permissions{Read: true, Write: true}),
		}

		err := host.Call(ctx, "call-complex-host-func", &result, input)
		require.NoError(t, err)

		expected := "Received request request-ABC with 2 data records. Write permission is enabled."
		assert.Equal(t, expected, result)
	})

	t.Run("Test Scalars", func(t *testing.T) {
		testGuestScalar(t, host, "u8", uint8(0), uint8(1), uint8(math.MaxUint8))
		testGuestScalar(t, host, "s8", int8(math.MinInt8), int8(-1), int8(0), int8(1), int8(math.MaxInt8))
		testGuestScalar(t, host, "u16", uint16(0), uint16(1), uint16(math.MaxUint16))
		testGuestScalar(t, host, "s16", int16(math.MinInt16), int16(-1), int16(0), int16(1), int16(math.MaxInt16))
		testGuestScalar(t, host, "u32", uint32(0), uint32(1), uint32(math.MaxUint32))
		testGuestScalar(t, host, "s32", int32(math.MinInt32), int32(-1), int32(0), int32(1), int32(math.MaxInt32))
		testGuestScalar(t, host, "u64", uint64(0), uint64(1), uint64(math.MaxUint64))
		testGuestScalar(t, host, "s64", int64(math.MinInt64), int64(-1), int64(0), int64(1), int64(math.MaxInt64))
		testGuestScalar(t, host, "float32", float32(-1), float32(0), float32(1), float32(math.SmallestNonzeroFloat32), float32(math.MaxFloat32))
		testGuestScalar(t, host, "float64", float64(-1), float64(0), float64(1), math.SmallestNonzeroFloat64, math.MaxFloat64)
		testGuestScalar(t, host, "bool", true, false)
	})

	t.Run("Verify Host Exports via Guest", func(t *testing.T) {
		// This single call triggers all the assertions within the guest's
		// `verify-host-scalars` function. If any rust `assert_eq!` fails,
		// the guest will trap, and `host.Call` will return an error.
		err := host.Call(ctx, "verify-host-scalars", nil)
		require.NoError(t, err, "guest trapped, indicating a failure in host export logic")
	})
}

func testGuestScalar[T any](t *testing.T, host *Host, name string, values ...T) {
	t.Run("guest-"+name, func(t *testing.T) {
		for _, value := range values {
			var result T
			err := host.Call(context.Background(), "test-"+name, &result, value)
			require.NoError(t, err)
			assert.Equal(t, value, result)
		}
	})

	t.Run("guest-option-"+name, func(t *testing.T) {
		for _, value := range values {
			var result Option[T]
			err := host.Call(context.Background(), "test-option-"+name, &result, Some(value))
			require.NoError(t, err)
			assert.Equal(t, value, *result.Some)
		}
	})

	t.Run("guest-result-"+name, func(t *testing.T) {
		for _, value := range values {
			var result Result[T, Unit]
			err := host.Call(context.Background(), "test-result-"+name, &result, Ok[T, Unit](value))
			require.NoError(t, err)
			assert.Equal(t, value, *result.Ok)
		}
	})
}
