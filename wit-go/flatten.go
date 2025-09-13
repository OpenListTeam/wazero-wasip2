package witgo

import (
	"context"
	"fmt"
	"math"
	"reflect"
)

// flattenParam recursively deconstructs a Go value into a slice of flat Wasm parameters.
func (h *Host) flattenParam(ctx context.Context, val reflect.Value) ([]uint64, error) {
	typ := val.Type()

	if isOption(typ) {
		return h.flattenOption(ctx, val)
	} else if isResult(typ) {
		return h.flattenResult(ctx, val)
	} else if isVariant(typ) {
		return h.flattenVariant(ctx, val)
	} else if isFlags(typ) {
		return h.flattenFlags(ctx, val)
	}

	switch val.Kind() {
	case reflect.String:
		return h.flattenString(ctx, val)
	case reflect.Struct: // This case is now for WIT records passed as parameters
		return h.flattenStruct(ctx, val)
	case reflect.Array:
		var flattened []uint64
		for i := 0; i < val.Len(); i++ {
			elemFlat, err := h.flattenParam(ctx, val.Index(i))
			if err != nil {
				return nil, fmt.Errorf("failed to flatten array element %d: %w", i, err)
			}
			flattened = append(flattened, elemFlat...)
		}
		return flattened, nil
	case reflect.Slice:
		return h.flattenSlice(ctx, val)
	case reflect.Bool:
		if val.Bool() {
			return []uint64{1}, nil
		}
		return []uint64{0}, nil
	case reflect.Float32:
		return []uint64{uint64(math.Float32bits(float32(val.Float())))}, nil
	case reflect.Float64:
		return []uint64{math.Float64bits(val.Float())}, nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return []uint64{val.Uint()}, nil
	default:
		return nil, fmt.Errorf("unsupported parameter kind for flattening: %v", val.Kind())
	}
}

// flattenStruct handles record flattening.
func (h *Host) flattenStruct(ctx context.Context, val reflect.Value) ([]uint64, error) {
	var flattened []uint64
	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)

		var fieldFlat []uint64
		var err error

		fieldFlat, err = h.flattenParam(ctx, fieldVal)
		if err != nil {
			return nil, fmt.Errorf("failed to flatten nested field %s: %w", val.Type().Field(i).Name, err)
		}
		flattened = append(flattened, fieldFlat...)
	}
	return flattened, nil
}

// flattenSlice handles lifting a Go slice to guest memory and returns its {ptr, len} representation.
func (h *Host) flattenSlice(ctx context.Context, val reflect.Value) ([]uint64, error) {
	if !val.IsValid() || val.IsZero() {
		return []uint64{0, 0}, nil
	}

	// This logic is analogous to the generic liftSlice, but it returns the
	// {ptr, len} pair directly for use as flattened parameters.
	liftedStructPtr, err := Lift(ctx, h.module.Memory(), h.allocator, val)
	if err != nil {
		return nil, fmt.Errorf("failed to lift slice: %w", err)
	}

	contentPtr, ok := h.module.Memory().ReadUint32Le(liftedStructPtr)
	if !ok {
		return nil, fmt.Errorf("failed to read content pointer for slice param at ptr %d", liftedStructPtr)
	}
	contentLen, ok := h.module.Memory().ReadUint32Le(liftedStructPtr + 4)
	if !ok {
		return nil, fmt.Errorf("failed to read content length for slice param at ptr %d", liftedStructPtr+4)
	}

	return []uint64{uint64(contentPtr), uint64(contentLen)}, nil
}

// flattenString directly allocates string content and returns {ptr, len}.
// This avoids the extra allocation and readback of the previous Lift-based approach.
func (h *Host) flattenString(ctx context.Context, val reflect.Value) ([]uint64, error) {
	if !val.IsValid() || val.IsZero() {
		return []uint64{0, 0}, nil
	}
	s := val.String()
	contentPtr, err := h.allocator.Allocate(ctx, uint32(len(s)), 1)
	if err != nil {
		return nil, err
	}
	if !h.module.Memory().Write(contentPtr, []byte(s)) {
		return nil, fmt.Errorf("failed to write string content for param")
	}
	return []uint64{uint64(contentPtr), uint64(len(s))}, nil
}

func (h *Host) flattenOption(ctx context.Context, val reflect.Value) ([]uint64, error) {
	hasValue := val.FieldByName("HasValue").Bool()
	valueField := val.FieldByName("Value")

	payloadShape, err := h.flattenParam(ctx, reflect.Zero(valueField.Type()))
	if err != nil {
		return nil, err
	}
	payloadLen := len(payloadShape)

	if hasValue {
		payload, err := h.flattenParam(ctx, valueField)
		if err != nil {
			return nil, err
		}
		return append([]uint64{1}, payload...), nil
	}
	return append([]uint64{0}, make([]uint64, payloadLen)...), nil
}

func (h *Host) flattenResult(ctx context.Context, val reflect.Value) ([]uint64, error) {
	isErr := val.FieldByName("IsErr").Bool()
	okField := val.FieldByName("Ok")
	errField := val.FieldByName("Err")

	okPayloadShape, err := h.flattenParam(ctx, reflect.Zero(okField.Type()))
	if err != nil {
		return nil, err
	}
	errPayloadShape, err := h.flattenParam(ctx, reflect.Zero(errField.Type()))
	if err != nil {
		return nil, err
	}

	maxPayloadLen := max(len(okPayloadShape), len(errPayloadShape))
	var finalPayload []uint64

	if isErr {
		errPayload, err := h.flattenParam(ctx, errField)
		if err != nil {
			return nil, err
		}
		finalPayload = append(errPayload, make([]uint64, maxPayloadLen-len(errPayload))...)
		return append([]uint64{1}, finalPayload...), nil
	}

	okPayload, err := h.flattenParam(ctx, okField)
	if err != nil {
		return nil, err
	}
	finalPayload = append(okPayload, make([]uint64, maxPayloadLen-len(okPayload))...)
	return append([]uint64{0}, finalPayload...), nil
}

func (h *Host) flattenVariant(ctx context.Context, val reflect.Value) ([]uint64, error) {
	typ := val.Type()
	numCases := val.NumField()
	var maxPayloadLen int
	payloadShapes := make([][]uint64, numCases)

	for i := 0; i < numCases; i++ {
		field := typ.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		shape, err := h.flattenParam(ctx, reflect.Zero(fieldType))
		if err != nil {
			return nil, fmt.Errorf("could not get shape for variant case %s: %w", field.Name, err)
		}
		payloadShapes[i] = shape
		if len(shape) > maxPayloadLen {
			maxPayloadLen = len(shape)
		}
	}

	if val.IsZero() {
		return make([]uint64, 1+maxPayloadLen), nil
	}

	for i := 0; i < numCases; i++ {
		fieldVal := val.Field(i)
		if !fieldVal.IsZero() {
			activeDiscriminant := uint64(i)
			activePayloadField := fieldVal
			if activePayloadField.Kind() == reflect.Pointer {
				activePayloadField = activePayloadField.Elem()
			}

			activePayload, err := h.flattenParam(ctx, activePayloadField)
			if err != nil {
				return nil, err
			}

			finalParams := []uint64{activeDiscriminant}
			finalParams = append(finalParams, activePayload...)
			padding := make([]uint64, maxPayloadLen-len(activePayload))
			finalParams = append(finalParams, padding...)

			return finalParams, nil
		}
	}
	return nil, fmt.Errorf("invalid variant: no case set for %v", typ)
}

func (h *Host) flattenFlags(ctx context.Context, val reflect.Value) ([]uint64, error) {
	var bits uint64
	for i := 0; i < val.NumField(); i++ {
		if val.Field(i).Bool() {
			bits |= (1 << i)
		}
	}
	return []uint64{bits}, nil
}
