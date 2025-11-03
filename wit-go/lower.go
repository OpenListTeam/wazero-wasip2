package witgo

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// Lower reads data from a pointer in guest memory into a Go value using a cached codec.
func Lower(ctx context.Context, h *Host, ptr uint32, goVal reflect.Value) error {
	l, err := getOrGenerateLowerer(goVal.Type())
	if err != nil {
		return fmt.Errorf("failed to get lowerer for type %v: %w", goVal.Type(), err)
	}
	return l.lower(ctx, h, ptr, goVal)
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

// read is now an internal helper used by codecs.
func read(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value, layout *TypeLayout) error {
	for val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

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
		layout, err := GetOrCalculateLayout(typ)
		if err != nil {
			return err
		}
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
	case reflect.Bool:
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for bool at ptr %d", ptr)
		}
		val.SetBool(b == 1)
		return nil
	case reflect.Int8:
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for byte at ptr %d", ptr)
		}
		val.SetInt(int64(b))
		return nil
	case reflect.Uint8:
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for byte at ptr %d", ptr)
		}
		val.SetUint(uint64(b))
		return nil
	case reflect.Int16:
		b, ok := mem.ReadUint16Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for int16 at ptr %d", ptr)
		}
		val.SetInt(int64(b))
		return nil
	case reflect.Uint16:
		i, ok := mem.ReadUint16Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for uint16 at ptr %d", ptr)
		}
		val.SetUint(uint64(i))
		return nil
	case reflect.Int32:
		b, ok := mem.ReadUint32Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for int32 at ptr %d", ptr)
		}
		val.SetInt(int64(b))
		return nil
	case reflect.Uint32:
		i, ok := mem.ReadUint32Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for uint32 at ptr %d", ptr)
		}
		val.SetUint(uint64(i))
		return nil
	case reflect.Int64:
		b, ok := mem.ReadUint64Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for int64 at ptr %d", ptr)
		}
		val.SetInt(int64(b))
		return nil
	case reflect.Uint64:
		i, ok := mem.ReadUint64Le(ptr)
		if !ok {
			return fmt.Errorf("memory read failed for uint64 at ptr %d", ptr)
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

func lowerSlice(ctx context.Context, mem api.Memory, ptr uint32, val reflect.Value) error {
	buf, ok := mem.Read(ptr, 8)
	if !ok {
		return fmt.Errorf("failed to read slice ptr/len at ptr %d", ptr)
	}
	contentPtr := binary.LittleEndian.Uint32(buf[0:4])
	contentLen := binary.LittleEndian.Uint32(buf[4:8])

	return lowerSlice2(ctx, mem, contentPtr, contentLen, val)
}

func lowerSlice2(ctx context.Context, mem api.Memory, contentPtr uint32, contentLen uint32, val reflect.Value) error {
	elemType := val.Type().Elem()

	// 快速路径：[]byte（元素类型为 uint8），一次性读取并直接赋值，避免逐元素读写开销。
	if elemType.Kind() == reflect.Uint8 {
		if contentLen == 0 {
			// 设置空切片
			var zero []byte
			val.Set(reflect.ValueOf(zero))
			return nil
		}
		content, err := LowerSliceFromParts(mem, contentPtr, contentLen)
		if err != nil {
			return err
		}
		val.Set(reflect.ValueOf(content))
		return nil
	}

	// 通用路径：逐元素使用已有布局读取
	elemLayout, err := GetOrCalculateLayout(elemType)
	if err != nil {
		return err
	}

	newSlice := reflect.MakeSlice(val.Type(), int(contentLen), int(contentLen))

	stride := align(elemLayout.Size, elemLayout.Alignment)

	for i := 0; i < int(contentLen); i++ {
		elemVal := newSlice.Index(i)
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
