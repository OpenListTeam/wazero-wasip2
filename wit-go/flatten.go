package witgo

import (
	"context"
	"fmt"
	"math"
	"reflect"
)

// flattenParam recursively deconstructs a Go value, appending the flat Wasm parameters to a slice.
func (h *Host) flattenParam(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	typ := val.Type()

	if isVariant(typ) {
		return h.flattenVariant(ctx, val, flatParams)
	}
	if isFlags(typ) {
		return h.flattenFlags(ctx, val, flatParams)
	}

	switch val.Kind() {
	case reflect.String:
		return h.flattenString(ctx, val, flatParams)
	case reflect.Slice:
		return h.flattenSlice(ctx, val, flatParams)
	case reflect.Struct:
		return h.flattenStruct(ctx, val, flatParams)
	case reflect.Array:
		return h.flattenArray(ctx, val, flatParams)
	case reflect.Bool:
		if val.Bool() {
			*flatParams = append(*flatParams, 1)
		} else {
			*flatParams = append(*flatParams, 0)
		}
		return nil
	case reflect.Float32:
		*flatParams = append(*flatParams, uint64(math.Float32bits(float32(val.Float()))))
		return nil
	case reflect.Float64:
		*flatParams = append(*flatParams, math.Float64bits(val.Float()))
		return nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*flatParams = append(*flatParams, val.Uint())
		return nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*flatParams = append(*flatParams, uint64(val.Int()))
		return nil
	default:
		return fmt.Errorf("unsupported parameter kind for flattening: %v", val.Kind())
	}
}

func (h *Host) flattenStruct(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		err := h.flattenParam(ctx, fieldVal, flatParams)
		if err != nil {
			return fmt.Errorf("failed to flatten field %s: %w", val.Type().Field(i).Name, err)
		}
	}
	return nil
}

func (h *Host) flattenString(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	if !val.IsValid() || val.IsZero() {
		*flatParams = append(*flatParams, 0, 0)
		return nil
	}
	s := val.String()
	contentPtr, err := h.allocator.Allocate(ctx, uint32(len(s)), 1)
	if err != nil {
		return err
	}
	if !h.module.Memory().Write(contentPtr, []byte(s)) {
		return fmt.Errorf("failed to write string content for param")
	}
	*flatParams = append(*flatParams, uint64(contentPtr), uint64(len(s)))
	return nil
}

func (h *Host) flattenSlice(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	if !val.IsValid() || val.IsZero() {
		*flatParams = append(*flatParams, 0, 0)
		return nil
	}
	liftedStructPtr, err := Lift(ctx, h, val)
	if err != nil {
		return fmt.Errorf("failed to lift slice for param: %w", err)
	}
	contentPtr, ok := h.module.Memory().ReadUint32Le(liftedStructPtr)
	if !ok {
		return fmt.Errorf("failed to read content pointer for slice param at ptr %d", liftedStructPtr)
	}
	contentLen, ok := h.module.Memory().ReadUint32Le(liftedStructPtr + 4)
	if !ok {
		return fmt.Errorf("failed to read content length for slice param at ptr %d", liftedStructPtr+4)
	}
	*flatParams = append(*flatParams, uint64(contentPtr), uint64(contentLen))
	return nil
}

func (h *Host) flattenArray(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	for i := 0; i < val.Len(); i++ {
		err := h.flattenParam(ctx, val.Index(i), flatParams)
		if err != nil {
			return fmt.Errorf("failed to flatten array element %d: %w", i, err)
		}
	}
	return nil
}

func (h *Host) flattenFlags(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	var bits uint64
	for i := 0; i < val.NumField(); i++ {
		if val.Field(i).Bool() {
			bits |= (1 << i)
		}
	}
	*flatParams = append(*flatParams, bits)
	return nil
}

func (h *Host) flattenVariant(ctx context.Context, val reflect.Value, flatParams *[]uint64) error {
	typ := val.Type()
	numCases := val.NumField()
	var maxPayloadLen int
	payloadShapes := make([][]uint64, numCases)

	for i := 0; i < numCases; i++ {
		var shape []uint64
		field := typ.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		err := h.flattenParam(ctx, reflect.Zero(fieldType), &shape)
		if err != nil {
			return fmt.Errorf("could not get shape for variant case %s: %w", field.Name, err)
		}
		payloadShapes[i] = shape
		if len(shape) > maxPayloadLen {
			maxPayloadLen = len(shape)
		}
	}

	if val.IsZero() {
		*flatParams = append(*flatParams, make([]uint64, 1+maxPayloadLen)...)
		return nil
	}

	for i := 0; i < numCases; i++ {
		fieldVal := val.Field(i)
		if !fieldVal.IsZero() {
			activeDiscriminant := uint64(i)
			activePayloadField := fieldVal
			if activePayloadField.Kind() == reflect.Pointer {
				activePayloadField = activePayloadField.Elem()
			}

			*flatParams = append(*flatParams, activeDiscriminant)
			startLen := len(*flatParams)
			err := h.flattenParam(ctx, activePayloadField, flatParams)
			if err != nil {
				return err
			}
			endLen := len(*flatParams)

			padding := make([]uint64, maxPayloadLen-(endLen-startLen))
			*flatParams = append(*flatParams, padding...)

			return nil
		}
	}
	return fmt.Errorf("invalid variant: no case set for %v", typ)
}
