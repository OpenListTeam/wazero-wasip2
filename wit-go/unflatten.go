package witgo

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// paramStream helps to read sequentially from the flattened parameter stack.
type paramStream struct {
	params []uint64
	pos    int
}

func (s *paramStream) Next() (uint64, bool) {
	if s.pos >= len(s.params) {
		return 0, false
	}
	p := s.params[s.pos]
	s.pos++
	return p, true
}

// unflattenParam is the inverse of flattenParam. It reconstructs a high-level Go value
// by consuming one or more flat values from a stream.
func (h *Host) unflattenParam(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	val := reflect.New(targetType).Elem() // Create a new value to populate.

	// Handle special types that are passed as a single pointer.
	if isVariant(targetType) || (targetType.Kind() == reflect.Struct && !isFlags(targetType)) {
		ptr, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for pointer to %v", targetType)
		}
		err := Lower(ctx, mem, uint32(ptr), val)
		return val, err
	}

	switch targetType.Kind() {
	case reflect.String:
		ptr, ok1 := ps.Next()
		length, ok2 := ps.Next()
		if !ok1 || !ok2 {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for string")
		}
		s, err := LowerStringFromParts(mem, uint32(ptr), uint32(length))
		if err != nil {
			return reflect.Value{}, err
		}
		val.SetString(s)

	case reflect.Slice:
		ptr, ok1 := ps.Next()
		length, ok2 := ps.Next()
		if !ok1 || !ok2 {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for slice")
		}
		// NOTE: This currently only supports []byte. A full implementation
		// would need to recursively unflatten each element into a new Go slice.
		if targetType.Elem().Kind() == reflect.Uint8 {
			sl, err := LowerSliceFromParts(mem, uint32(ptr), uint32(length))
			if err != nil {
				return reflect.Value{}, err
			}
			val.SetBytes(sl)
		} else {
			return reflect.Value{}, fmt.Errorf("unflattening non-byte slices is not yet implemented")
		}

	case reflect.Bool:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for bool")
		}
		val.SetBool(p != 0)

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for %v", targetType)
		}
		val.SetInt(int64(p))

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for %v", targetType)
		}
		val.SetUint(p)

	case reflect.Float32:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for float32")
		}
		val.SetFloat(float64(math.Float32frombits(uint32(p))))

	case reflect.Float64:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for float64")
		}
		val.SetFloat(math.Float64frombits(p))

	default:
		return reflect.Value{}, fmt.Errorf("unsupported type for unflattening: %v", targetType.Kind())
	}

	return val, nil
}
