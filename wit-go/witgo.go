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

	var flatParams []uint64

	// flattenParam recursively deconstructs a Go value into a slice of flat Wasm parameters.
	var flattenParam func(val reflect.Value) ([]uint64, error)
	flattenParam = func(val reflect.Value) ([]uint64, error) {
		typ := val.Type()

		// --- Handle special sum types that need flattening ---
		if isOption(typ) {
			hasValue := val.FieldByName("HasValue").Bool()
			valueField := val.FieldByName("Value")

			// Recursively flatten the inner type to know its parameter shape.
			payload, err := flattenParam(valueField)
			if err != nil {
				return nil, err
			}

			if hasValue {
				return append([]uint64{1}, payload...), nil // {1, payload...}
			} else {
				return append([]uint64{0}, make([]uint64, len(payload))...), nil // {0, zeroed_payload...}
			}
		}

		if isResult(typ) {
			isErr := val.FieldByName("IsErr").Bool()
			okField := val.FieldByName("Ok")
			errField := val.FieldByName("Err")

			// Get the flattened shape of both payloads to determine the size of the payload area.
			okPayloadShape, err := flattenParam(reflect.Zero(okField.Type()))
			if err != nil {
				return nil, err
			}
			errPayloadShape, err := flattenParam(reflect.Zero(errField.Type()))
			if err != nil {
				return nil, err
			}

			maxPayloadLen := max(len(okPayloadShape), len(errPayloadShape))
			var finalPayload []uint64

			if isErr {
				errPayload, err := flattenParam(errField)
				if err != nil {
					return nil, err
				}
				// Pad the error payload to the max length
				finalPayload = append(errPayload, make([]uint64, maxPayloadLen-len(errPayload))...)
				return append([]uint64{1}, finalPayload...), nil
			} else {
				okPayload, err := flattenParam(okField)
				if err != nil {
					return nil, err
				}
				// Pad the ok payload to the max length
				finalPayload = append(okPayload, make([]uint64, maxPayloadLen-len(okPayload))...)
				return append([]uint64{0}, finalPayload...), nil
			}
		}

		if isVariant(typ) {
			numCases := val.NumField()
			var maxPayloadLen int
			payloadShapes := make([][]uint64, numCases)

			// First, determine the shape of all possible payloads to find the max length.
			for i := 0; i < numCases; i++ {
				field := typ.Field(i)
				fieldType := field.Type
				if fieldType.Kind() == reflect.Pointer {
					fieldType = fieldType.Elem()
				}
				shape, err := flattenParam(reflect.Zero(fieldType))
				if err != nil {
					return nil, fmt.Errorf("could not get shape for variant case %s: %w", field.Name, err)
				}
				payloadShapes[i] = shape
				if len(shape) > maxPayloadLen {
					maxPayloadLen = len(shape)
				}
			}

			// Now, find the active case and flatten its payload.
			for i := 0; i < numCases; i++ {
				fieldVal := val.Field(i)
				if !fieldVal.IsZero() {
					// This is the active case.
					activeDiscriminant := uint64(i)

					activePayloadField := fieldVal
					if activePayloadField.Kind() == reflect.Pointer {
						activePayloadField = activePayloadField.Elem()
					}

					activePayload, err := flattenParam(activePayloadField)
					if err != nil {
						return nil, err
					}

					// Build the final flattened parameters: discriminant + payload + padding.
					finalParams := []uint64{activeDiscriminant}
					finalParams = append(finalParams, activePayload...)
					// Add padding to match the longest possible payload.
					padding := make([]uint64, maxPayloadLen-len(activePayload))
					finalParams = append(finalParams, padding...)

					return finalParams, nil
				}
			}
			return nil, fmt.Errorf("invalid variant: no case set for %v", typ)
		}

		// --- Handle records and primitive types ---
		switch val.Kind() {
		case reflect.String:
			// For a zero-value string (e.g. from a None), no allocation is needed.
			if !val.IsValid() || val.IsZero() {
				return []uint64{0, 0}, nil
			}
			s := val.String()
			// Lift the string content to memory
			contentPtr, err := h.allocator.Allocate(ctx, uint32(len(s)), 1)
			if err != nil {
				return nil, err
			}
			if !h.module.Memory().Write(contentPtr, []byte(s)) {
				return nil, fmt.Errorf("failed to write string content")
			}
			return []uint64{uint64(contentPtr), uint64(len(s))}, nil

		case reflect.Struct: // This case is now only for WIT records
			ptr, err := Lift(ctx, h.module.Memory(), h.allocator, val)
			if err != nil {
				return nil, err
			}
			return []uint64{uint64(ptr)}, nil
		case reflect.Array:
			var flattened []uint64
			for i := 0; i < val.Len(); i++ {
				elemFlat, err := flattenParam(val.Index(i))
				if err != nil {
					return nil, fmt.Errorf("failed to flatten array element %d: %w", i, err)
				}
				flattened = append(flattened, elemFlat...)
			}
			return flattened, nil
		case reflect.Uint32, reflect.Uint8, reflect.Uint16:
			return []uint64{val.Uint()}, nil

		case reflect.Float32:
			return []uint64{uint64(math.Float32bits(float32(val.Float())))}, nil
		case reflect.Float64:
			return []uint64{math.Float64bits(val.Float())}, nil

		case reflect.Bool:
			if val.Bool() {
				return []uint64{1}, nil
			}
			return []uint64{0}, nil

		default:
			return nil, fmt.Errorf("unsupported parameter kind for flattening: %v", val.Kind())
		}
	}

	for _, p := range params {
		pFlat, err := flattenParam(reflect.ValueOf(p))
		if err != nil {
			return fmt.Errorf("failed to flatten parameter %#v: %w", p, err)
		}
		flatParams = append(flatParams, pFlat...)
	}
	// Execute the raw Wasm call.
	// CRITICAL FIX: No `retptr` is ever passed as a parameter for single-return functions.
	results, err := fn.Call(ctx, flatParams...)
	if err != nil {
		return fmt.Errorf("guest function '%s' call failed: %w", funcName, err)
	}

	// Lower the result from Wasm memory back into the Go result pointer.
	if resultPtr != nil {
		// If the guest returns a value, it will be a single pointer to the result data.
		if len(results) < 1 {
			// Check if the function was actually supposed to return something.
			if len(fn.Definition().ResultNames()) > 0 {
				return fmt.Errorf("expected at least 1 return value (a pointer), but got %d", len(results))
			}
			// It was a void function, nothing to do.
			return nil
		}

		resultValuePtr := uint32(results[0])
		outVal := reflect.ValueOf(resultPtr).Elem()

		err = Lower(ctx, h.module.Memory(), resultValuePtr, outVal)
		if err != nil {
			return fmt.Errorf("failed to lower result from ptr %d: %w", resultValuePtr, err)
		}
	}

	return nil
}
