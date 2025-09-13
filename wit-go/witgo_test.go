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

func TestWitGo(t *testing.T) {
	ctx := context.Background()

	// 1. Create a wazero runtime.
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// 2. Instantiate WASI, which is required for wasm32-wasi modules.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// 3. Create a host module to provide imported functions to the guest.
	var hostLogBuffer string
	_, err := r.NewHostModuleBuilder("$root").
		NewFunctionBuilder().
		// This function signature matches the flattened ABI for a string parameter: (ptr, len).
		WithFunc(func(ctx context.Context, ptr, len uint32) {
			// Get a reference to the guest's memory.
			mem := r.Module("test-instance").Memory()
			msg, ok := mem.Read(ptr, len)
			require.True(t, ok, "failed to read imported string from guest memory")

			// Store and print the message.
			hostLogBuffer = string(msg)
			fmt.Printf("[HOST LOG]: %s\n", hostLogBuffer)
		}).
		Export("host-log").
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

		require.True(t, result.HasValue)
		assert.Equal(t, uint32(len("this is a test")), result.Value)
	})

	t.Run("Roundtrip Option: None value", func(t *testing.T) {
		var result Option[uint32]
		input := None[string]()
		err := host.Call(ctx, "handle-option", &result, input)
		require.NoError(t, err)
		assert.False(t, result.HasValue)
	})

	t.Run("Roundtrip Result: Ok value", func(t *testing.T) {
		var result Result[uint32, Color]
		input := Ok[string, Color]("success")
		err := host.Call(ctx, "handle-result", &result, input)
		require.NoError(t, err)

		require.False(t, result.IsErr)
		assert.Equal(t, uint32(len("success")), result.Ok)
	})

	t.Run("Roundtrip Result: Err value", func(t *testing.T) {
		var result Result[uint32, Color]
		input := Err[string, Color](ColorBlue)
		err := host.Call(ctx, "handle-result", &result, input)
		require.NoError(t, err)

		require.True(t, result.IsErr)
		assert.Equal(t, ColorBlue, result.Err)
	})

	t.Run("Host to Guest: Variant", func(t *testing.T) {
		var result string
		radius := float32(10.5)
		input := Shape{Circle: radius} // Pass a circle variant

		err := host.Call(ctx, "handle-variant", &result, input)
		require.NoError(t, err)
		assert.Equal(t, "Circle with radius 10.5", result)
	})
}
