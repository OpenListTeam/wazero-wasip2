package witgo

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
)

// Lift writes a Go value into guest memory according to its layout and returns the pointer.
func Lift(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value) (uint32, error) {
	layout, err := GetOrCalculateLayout(val.Type())
	if err != nil {
		return 0, err
	}

	ptr, err := alloc.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}

	if err := write(ctx, mem, alloc, val, ptr, layout); err != nil {
		return 0, err
	}
	return ptr, nil
}

func write(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32, layout *TypeLayout) error {
	typ := val.Type()

	if isOption(typ) {
		hasValue := val.FieldByName("HasValue").Bool() // Changed from "hasValue"
		if !mem.WriteByte(ptr, boolToByte(hasValue)) {
			return fmt.Errorf("failed to write option discriminant")
		}
		if hasValue {
			valueField := val.FieldByName("Value") // Changed from "value"
			valueLayout, err := GetOrCalculateLayout(valueField.Type())
			if err != nil {
				return err
			}
			payloadOffset := align(1, valueLayout.Alignment)
			return write(ctx, mem, alloc, valueField, ptr+payloadOffset, valueLayout)
		}
		return nil
	}
	if isResult(typ) {
		okField := val.FieldByName("Ok")
		errField := val.FieldByName("Err")
		okLayout, err := GetOrCalculateLayout(okField.Type())
		if err != nil {
			return err
		}
		errLayout, err := GetOrCalculateLayout(errField.Type())
		if err != nil {
			return err
		}

		maxAlign := okLayout.Alignment
		if errLayout.Alignment > maxAlign {
			maxAlign = errLayout.Alignment
		}
		payloadOffset := align(1, maxAlign) //disc size is 1

		isErr := val.FieldByName("IsErr").Bool()
		if !mem.WriteByte(ptr, boolToByte(isErr)) {
			return fmt.Errorf("failed to write result discriminant")
		}

		if isErr {
			return write(ctx, mem, alloc, errField, ptr+payloadOffset, errLayout)
		}
		return write(ctx, mem, alloc, okField, ptr+payloadOffset, okLayout)
	}

	if isVariant(typ) {
		var maxAlign uint32 = 1
		caseLayouts := make([]*TypeLayout, val.NumField())
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			fieldType := field.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			layout, err := GetOrCalculateLayout(fieldType)
			if err != nil {
				return err
			}
			caseLayouts[i] = layout
			if layout.Alignment > maxAlign {
				maxAlign = layout.Alignment
			}
		}
		payloadOffset := align(1, maxAlign) // disc size is 1

		for i := 0; i < val.NumField(); i++ {
			fieldVal := val.Field(i)
			if !fieldVal.IsZero() {
				if !mem.WriteByte(ptr, byte(i)) {
					return fmt.Errorf("failed to write variant discriminant")
				}

				payloadField := fieldVal
				if payloadField.Kind() == reflect.Pointer {
					payloadField = payloadField.Elem()
				}
				return write(ctx, mem, alloc, payloadField, ptr+payloadOffset, caseLayouts[i])
			}
		}
		return fmt.Errorf("invalid variant: no case set")
	}

	switch val.Kind() {
	case reflect.Uint8, reflect.Int8:
		if !mem.WriteByte(ptr, byte(val.Uint())) {
			return fmt.Errorf("memory write failed for byte at ptr %d", ptr)
		}
		return nil
	case reflect.Bool:
		if !mem.WriteByte(ptr, boolToByte(val.Bool())) {
			return fmt.Errorf("memory write failed for bool at ptr %d", ptr)
		}
		return nil
	case reflect.Uint32, reflect.Int32:
		if !mem.WriteUint32Le(ptr, uint32(val.Uint())) {
			return fmt.Errorf("memory write failed for uint32 at ptr %d", ptr)
		}
		return nil
	case reflect.Float32:
		if !mem.WriteFloat32Le(ptr, float32(val.Float())) {
			return fmt.Errorf("memory write failed for float32 at ptr %d", ptr)
		}
		return nil
	case reflect.Float64:
		if !mem.WriteFloat64Le(ptr, val.Float()) {
			return fmt.Errorf("memory write failed for float64 at ptr %d", ptr)
		}
		return nil
	case reflect.String:
		return liftString(ctx, mem, alloc, val.String(), ptr)
	case reflect.Slice:
		return liftSlice(ctx, mem, alloc, val, ptr)
	case reflect.Struct:
		return liftStruct(ctx, mem, alloc, val, ptr, layout)
	case reflect.Array:
		return liftArray(ctx, mem, alloc, val, ptr)
	default:
		return fmt.Errorf("unsupported type for lifting: %v", val.Kind())
	}
}

func liftString(ctx context.Context, mem api.Memory, alloc *GuestAllocator, s string, ptr uint32) error {
	contentPtr, err := alloc.Allocate(ctx, uint32(len(s)), 1)
	if err != nil {
		return err
	}
	if !mem.Write(contentPtr, []byte(s)) {
		return fmt.Errorf("memory write failed for string content at ptr %d", contentPtr)
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], contentPtr)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(s)))
	if !mem.Write(ptr, buf) {
		return fmt.Errorf("memory write failed for string header at ptr %d", ptr)
	}
	return nil
}

func liftSlice(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32) error {
	elemLayout, err := GetOrCalculateLayout(val.Type().Elem())
	if err != nil {
		return err
	}
	sliceLen := val.Len()
	contentSize := uint32(sliceLen) * elemLayout.Size

	contentPtr, err := alloc.Allocate(ctx, contentSize, elemLayout.Alignment)
	if err != nil {
		return err
	}

	if elemLayout.Size == 1 { // Handle byte slices directly
		header := (*reflect.SliceHeader)(unsafe.Pointer(val.Addr().UnsafePointer()))
		data := unsafe.Slice((*byte)(unsafe.Pointer(header.Data)), header.Len)
		if !mem.Write(contentPtr, data) {
			return fmt.Errorf("memory write failed for slice content at ptr %d", contentPtr)
		}
	} else {
		return fmt.Errorf("lifting non-byte slices requires recursive writes (not yet implemented)")
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], contentPtr)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(sliceLen))
	if !mem.Write(ptr, buf) {
		return fmt.Errorf("memory write failed for slice header at ptr %d", ptr)
	}
	return nil
}

func liftArray(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32) error {
	elemLayout, err := GetOrCalculateLayout(val.Type().Elem())
	if err != nil {
		return err
	}
	currentOffset := ptr
	for i := 0; i < val.Len(); i++ {
		elemVal := val.Index(i)
		elemPtr := align(currentOffset, elemLayout.Alignment)
		if err := write(ctx, mem, alloc, elemVal, elemPtr, elemLayout); err != nil {
			return fmt.Errorf("failed to write array element %d: %w", i, err)
		}
		currentOffset = elemPtr + elemLayout.Size
	}
	return nil
}

func liftStruct(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32, layout *TypeLayout) error {
	for _, fieldLayout := range layout.Fields {
		fieldVal := val.FieldByName(fieldLayout.StructField.Name)
		fieldPtr := ptr + fieldLayout.Offset
		if err := write(ctx, mem, alloc, fieldVal, fieldPtr, fieldLayout.Layout); err != nil {
			return err
		}
	}
	return nil
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
