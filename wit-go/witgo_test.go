package witgo

import (
	"context"
	_ "embed"
	"fmt"
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

func TestWitGo(t *testing.T) {
	ctx := context.Background()

	// 1. Create a wazero runtime.
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// 2. Instantiate WASI, which is required for wasm32-wasi modules.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// 3. Create a host module to provide imported functions to the guest.
	var hostLogBuffer string
	// _, err := r.NewHostModuleBuilder("$root").
	// 	NewFunctionBuilder().
	// 	// This function signature matches the flattened ABI for a string parameter: (ptr, len).
	// 	WithFunc(func(ctx context.Context, ptr, len uint32) {
	// 		// Get a reference to the guest's memory.
	// 		mem := r.Module("test-instance").Memory()
	// 		msg, ok := mem.Read(ptr, len)
	// 		require.True(t, ok, "failed to read imported string from guest memory")

	// 		// Store and print the message.
	// 		hostLogBuffer = string(msg)
	// 		fmt.Printf("[HOST LOG]: %s\n", hostLogBuffer)
	// 	}).
	// 	Export("host-log").
	// 	Instantiate(ctx)
	// require.NoError(t, err)

	// 1. Create an Exporter that wraps the wazero builder.
	exporter := NewExporter(r.NewHostModuleBuilder("$root"))

	// 2. Use the chainable, "Must" variant to export functions.
	_, err := exporter.
		MustExport("host-log", func(msg string) {
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
}
