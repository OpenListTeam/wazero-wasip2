package witgo

import (
	"fmt"
	"reflect"
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

// maxType correctly determines the result type when merging variant cases, now handling floats.
func maxType(a, b reflect.Type) reflect.Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a == b {
		return a
	}

	aKind := a.Kind()
	bKind := b.Kind()

	// WIT ABI does not mix floats and ints in the same variant slot.
	// We prioritize float types to fix the bug where they were being ignored.
	if aKind == reflect.Float64 || bKind == reflect.Float64 {
		return reflect.TypeFor[float64]()
	}
	if aKind == reflect.Float32 || bKind == reflect.Float32 {
		return reflect.TypeFor[float32]()
	}

	// Handle integer promotion (32-bit to 64-bit).
	aIs64 := aKind == reflect.Int64 || aKind == reflect.Uint64
	bIs64 := bKind == reflect.Int64 || bKind == reflect.Uint64
	if aIs64 || bIs64 {
		return reflect.TypeFor[int64]()
	}

	if aKind == reflect.Int32 || bKind == reflect.Int32 {
		return reflect.TypeFor[int32]()
	}
	return reflect.TypeFor[uint32]()
}

// maxFlat merges multiple flattened type lists into one, using the corrected maxType logic.
func maxFlat(lists ...[]reflect.Type) []reflect.Type {
	maxLength := 0
	for _, l := range lists {
		if len(l) > maxLength {
			maxLength = len(l)
		}
	}
	if maxLength == 0 {
		return nil
	}

	result := make([]reflect.Type, maxLength) // Initialize with nils

	for _, l := range lists {
		for i, t := range l {
			result[i] = maxType(result[i], t)
		}
	}
	return result
}

type unflattenState struct {
	flat   []uint64
	offset int
}

func (u *unflattenState) unflatten() (uint64, error) {
	if u.offset >= len(u.flat) {
		return 0, fmt.Errorf("not enough values to unflatten")
	}
	val := u.flat[u.offset]
	u.offset++
	return val, nil
}
