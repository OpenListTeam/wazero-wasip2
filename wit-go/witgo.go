package witgo

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// Host provides a high-level interface to interact with a Wasm component.
type Host struct {
	module    api.Module
	allocator *GuestAllocator
}

// NewHost creates a new Host instance for the given Wasm module.
func NewHost(module api.Module) (*Host, error) {
	alloc, err := NewGuestAllocator(module)
	if err != nil {
		return nil, err
	}
	return &Host{module: module, allocator: alloc}, nil
}

// Call an exported guest function.
// It automatically handles lifting arguments and lowering the result.
func (h *Host) Call(ctx context.Context, funcName string, resultPtr interface{}, params ...interface{}) error {
	fn := h.module.ExportedFunction(funcName)
	if fn == nil {
		return fmt.Errorf("function '%s' not found in guest exports", funcName)
	}

	flatParams := make([]uint64, 0, 16)
	for _, p := range params {
		// Pass a pointer to the slice to be appended to.
		err := h.flattenParam(ctx, reflect.ValueOf(p), &flatParams)
		if err != nil {
			return fmt.Errorf("failed to flatten parameter %#v: %w", p, err)
		}
	}

	// Execute the raw Wasm call.
	// CRITICAL FIX: No `retptr` is ever passed as a parameter for single-return functions.
	results, err := fn.Call(ctx, flatParams...)
	if err != nil {
		return fmt.Errorf("guest function '%s' call failed: %w", funcName, err)
	}

	// Lower the result from Wasm memory back into the Go result pointer.
	if resultPtr != nil {
		if len(results) == 0 {
			if len(fn.Definition().ResultNames()) > 0 {
				return fmt.Errorf("function was expected to return a value, but returned none")
			}
			return nil // It was a void function, which is fine.
		}

		outVal := reflect.ValueOf(resultPtr).Elem()
		resultValue := results[0]

		switch outVal.Kind() {
		// Scalar types are returned directly as values.
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			outVal.SetUint(resultValue)
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			outVal.SetInt(int64(resultValue))
		case reflect.Float32:
			outVal.SetFloat(float64(math.Float32frombits(uint32(resultValue))))

		case reflect.Float64:
			outVal.SetFloat(math.Float64frombits(resultValue))

		case reflect.Bool:
			outVal.SetBool(resultValue != 0)

		default:
			// For complex types (struct, string, slice, etc.), the return value is a pointer.
			ptr := uint32(resultValue)
			err = Lower(ctx, h.module.Memory(), ptr, outVal)
			if err != nil {
				return fmt.Errorf("failed to lower complex result from ptr %d: %w", ptr, err)
			}
		}
	}

	return nil
}
