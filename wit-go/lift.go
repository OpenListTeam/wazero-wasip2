package witgo

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// Lift writes a Go value into guest memory using a cached codec and returns the pointer.
func Lift(ctx context.Context, h *Host, val reflect.Value) (uint32, error) {
	layout, err := GetOrCalculateLayout(val.Type())
	if err != nil {
		return 0, err
	}
	l, err := getOrGenerateLifter(val.Type())
	if err != nil {
		return 0, fmt.Errorf("failed to get lifter for type %v: %w", val.Type(), err)
	}
	return l.lift(ctx, h, val, layout)
}

// LiftToPtr writes a Go value into a pre-allocated pointer in guest memory.
func LiftToPtr(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32) error {
	layout, err := GetOrCalculateLayout(val.Type())
	if err != nil {
		return err
	}
	return write(ctx, mem, alloc, val, ptr, layout)
}

// write is now an internal helper used by codecs.
func write(ctx context.Context, mem api.Memory, alloc *GuestAllocator, val reflect.Value, ptr uint32, layout *TypeLayout) error {
	typ := val.Type()

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
	} else if isFlags(typ) {
		var bits uint64
		for i := 0; i < val.NumField(); i++ {
			if val.Field(i).Bool() {
				bits |= (1 << i)
			}
		}
		switch layout.Size {
		case 1:
			return check(mem.WriteByte(ptr, byte(bits)))
		case 2:
			return check(mem.WriteUint16Le(ptr, uint16(bits)))
		case 4:
			return check(mem.WriteUint32Le(ptr, uint32(bits)))
		case 8:
			return check(mem.WriteUint64Le(ptr, bits))
		}
	}

	switch val.Kind() {
	case reflect.Bool:
		if !mem.WriteByte(ptr, boolToByte(val.Bool())) {
			return fmt.Errorf("memory write failed for bool at ptr %d", ptr)
		}
		return nil
	case reflect.Int8:
		if !mem.WriteByte(ptr, byte(val.Int())) {
			return fmt.Errorf("memory write failed for byte at ptr %d", ptr)
		}
		return nil
	case reflect.Uint8:
		if !mem.WriteByte(ptr, byte(val.Uint())) {
			return fmt.Errorf("memory write failed for byte at ptr %d", ptr)
		}
		return nil
	case reflect.Int16:
		if !mem.WriteUint16Le(ptr, uint16(val.Int())) {
			return fmt.Errorf("memory write failed for int16 at ptr %d", ptr)
		}
		return nil
	case reflect.Uint16:
		if !mem.WriteUint16Le(ptr, uint16(val.Uint())) {
			return fmt.Errorf("memory write failed for uint16 at ptr %d", ptr)
		}
		return nil
	case reflect.Int32:
		if !mem.WriteUint32Le(ptr, uint32(val.Int())) {
			return fmt.Errorf("memory write failed for int32 at ptr %d", ptr)
		}
		return nil
	case reflect.Uint32:
		if !mem.WriteUint32Le(ptr, uint32(val.Uint())) {
			return fmt.Errorf("memory write failed for uint32 at ptr %d", ptr)
		}
		return nil
	case reflect.Int64:
		if !mem.WriteUint64Le(ptr, uint64(val.Int())) {
			return fmt.Errorf("memory write failed for int64 at ptr %d", ptr)
		}
		return nil
	case reflect.Uint64:
		if !mem.WriteUint64Le(ptr, uint64(val.Uint())) {
			return fmt.Errorf("memory write failed for uint64 at ptr %d", ptr)
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

	stride := align(elemLayout.Size, elemLayout.Alignment)

	var contentSize uint32
	if sliceLen > 0 {
		contentSize = (uint32(sliceLen-1) * stride) + elemLayout.Size
	}

	contentPtr, err := alloc.Allocate(ctx, contentSize, elemLayout.Alignment)
	if err != nil {
		return err
	}

	for i := 0; i < sliceLen; i++ {
		elemVal := val.Index(i)
		elemPtr := contentPtr + (uint32(i) * stride)

		if err := write(ctx, mem, alloc, elemVal, elemPtr, elemLayout); err != nil {
			return fmt.Errorf("failed to write slice element %d: %w", i, err)
		}
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], contentPtr)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(sliceLen))
	return check(mem.Write(ptr, buf))
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

func check(ok bool) error {
	if !ok {
		return fmt.Errorf("memory access failed")
	}
	return nil
}
