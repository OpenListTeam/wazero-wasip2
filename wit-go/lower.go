package witgo

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// Lower reads data from a pointer in guest memory into a Go value.
func Lower(ctx context.Context, mem api.Memory, ptr uint32, goVal reflect.Value) error {
	layout, err := GetOrCalculateLayout(goVal.Type())
	if err != nil {
		return err
	}
	return read(ctx, mem, ptr, goVal, layout)
}

// LowerStringFromParts reads a string from guest memory given a direct pointer and length.
func LowerStringFromParts(mem api.Memory, ptr, length uint32) (string, error) {
	content, ok := mem.Read(ptr, length)
	if !ok {
		return "", fmt.Errorf("failed to read string content at ptr %d with length %d", ptr, length)
	}
	return string(content), nil
}

// LowerSliceFromParts reads a byte slice from guest memory given a direct pointer and length.
func LowerSliceFromParts(mem api.Memory, ptr, length uint32) ([]byte, error) {
	content, ok := mem.Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("failed to read slice content at ptr %d with length %d", ptr, length)
	}
	return content, nil
}

func read(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value, layout *TypeLayout) error {
	typ := val.Type()

	if isOption(typ) {
		disc, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("failed to read option discriminant")
		}
		val.FieldByName("HasValue").SetBool(disc == 1) // Changed from "hasValue"
		if disc == 1 {
			valueField := val.FieldByName("Value") // Changed from "value"
			valueLayout, err := GetOrCalculateLayout(valueField.Type())
			if err != nil {
				return err
			}
			payloadOffset := align(1, valueLayout.Alignment)
			return read(ctx, mem, ptr+payloadOffset, valueField, valueLayout)
		}
		return nil
	} else if isResult(typ) {
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
		payloadOffset := align(1, maxAlign)

		disc, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("failed to read result discriminant")
		}
		isErr := (disc == 1)
		val.FieldByName("IsErr").SetBool(isErr)

		if isErr {
			return read(ctx, mem, ptr+payloadOffset, errField, errLayout)
		}
		return read(ctx, mem, ptr+payloadOffset, okField, okLayout)
	} else if isVariant(typ) {
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
		payloadOffset := align(1, maxAlign)

		disc, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("failed to read variant discriminant")
		}

		field := typ.Field(int(disc))
		payloadField := val.FieldByName(field.Name)

		if payloadField.Kind() == reflect.Pointer {
			payloadField.Set(reflect.New(payloadField.Type().Elem()))
			payloadField = payloadField.Elem()
		}

		return read(ctx, mem, ptr+payloadOffset, payloadField, caseLayouts[disc])
	} else if isFlags(typ) {
		var bits uint64
		var ok bool
		switch layout.Size {
		case 1:
			var b byte
			b, ok = mem.ReadByte(ptr)
			bits = uint64(b)
		case 2:
			var s uint16
			s, ok = mem.ReadUint16Le(ptr)
			bits = uint64(s)
		case 4:
			var i uint32
			i, ok = mem.ReadUint32Le(ptr)
			bits = uint64(i)
		case 8:
			bits, ok = mem.ReadUint64Le(ptr)
		}
		if !ok {
			return fmt.Errorf("memory read failed for flags at ptr %d", ptr)
		}

		for i := 0; i < val.NumField(); i++ {
			if (bits & (1 << i)) != 0 {
				val.Field(i).SetBool(true)
			} else {
				val.Field(i).SetBool(false)
			}
		}
		return nil
	}
	switch val.Kind() {
	case reflect.Uint8, reflect.Int8:
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for byte at ptr %d", ptr)
		}
		val.SetUint(uint64(b))
		return nil
	case reflect.Bool:
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for bool at ptr %d", ptr)
		}
		val.SetBool(b == 1)
		return nil

	case reflect.Uint32, reflect.Int32:
		i, ok := mem.ReadUint32Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for uint32 at ptr %d", ptr)
		}
		val.SetUint(uint64(i))
		return nil
	case reflect.Float32:
		f, ok := mem.ReadFloat32Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for float32 at ptr %d", ptr)
		}
		val.SetFloat(float64(f))
		return nil
	case reflect.Float64:
		f, ok := mem.ReadFloat64Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for float64 at ptr %d", ptr)
		}
		val.SetFloat(f)
		return nil
	case reflect.String:
		s, err := lowerString(mem, ptr)
		if err != nil {
			return err
		}
		val.SetString(s)
		return nil
	case reflect.Slice:
		return lowerSlice(ctx, mem, ptr, val)
	case reflect.Struct:
		return lowerStruct(ctx, mem, ptr, val, layout)
	case reflect.Array:
		return lowerArray(ctx, mem, ptr, val)
	default:
		return fmt.Errorf("unsupported type for lowering: %v", val.Kind())
	}
}

func lowerString(mem api.Memory, ptr uint32) (string, error) {
	buf, ok := mem.Read(ptr, 8)
	if !ok {
		return "", fmt.Errorf("failed to read string ptr/len at ptr %d", ptr)
	}
	contentPtr := binary.LittleEndian.Uint32(buf[0:4])
	contentLen := binary.LittleEndian.Uint32(buf[4:8])
	return LowerStringFromParts(mem, contentPtr, contentLen)
}

// lowerSlice is now generic for any element type.
func lowerSlice(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value) error {
	buf, ok := mem.Read(ptr, 8)
	if !ok {
		return fmt.Errorf("failed to read slice ptr/len at ptr %d", ptr)
	}
	contentPtr := binary.LittleEndian.Uint32(buf[0:4])
	contentLen := binary.LittleEndian.Uint32(buf[4:8])

	elemType := val.Type().Elem()
	elemLayout, err := GetOrCalculateLayout(elemType)
	if err != nil {
		return err
	}

	newSlice := reflect.MakeSlice(val.Type(), int(contentLen), int(contentLen))

	// Stride is the size of each element "slot" in the array, including padding.
	stride := align(elemLayout.Size, elemLayout.Alignment)

	// Read each element recursively from the correct stride-based offset.
	for i := 0; i < int(contentLen); i++ {
		elemVal := newSlice.Index(i)
		// Calculate the precise starting pointer for the current element.
		elemPtr := contentPtr + (uint32(i) * stride)

		if err := read(ctx, mem, elemPtr, elemVal, elemLayout); err != nil {
			return fmt.Errorf("failed to read slice element %d: %w", i, err)
		}
	}

	val.Set(newSlice)
	return nil
}

func lowerArray(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value) error {
	elemLayout, err := GetOrCalculateLayout(val.Type().Elem())
	if err != nil {
		return err
	}
	currentOffset := ptr
	for i := 0; i < val.Len(); i++ {
		elemVal := val.Index(i)
		elemPtr := align(currentOffset, elemLayout.Alignment)
		if err := read(ctx, mem, elemPtr, elemVal, elemLayout); err != nil {
			return fmt.Errorf("failed to read array element %d: %w", i, err)
		}
		currentOffset = elemPtr + elemLayout.Size
	}
	return nil
}

func lowerStruct(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value, layout *TypeLayout) error {
	for _, fieldLayout := range layout.Fields {
		fieldVal := val.FieldByName(fieldLayout.StructField.Name)
		if !fieldVal.CanSet() {
			continue
		}
		fieldPtr := ptr + fieldLayout.Offset
		if err := read(ctx, mem, fieldPtr, fieldVal, fieldLayout.Layout); err != nil {
			return err
		}
	}
	return nil
}
