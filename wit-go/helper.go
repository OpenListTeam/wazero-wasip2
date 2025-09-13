package witgo

import (
	"reflect"
	"strings"
)

// align aligns ptr to the given alignment.
func align(ptr, alignment uint32) uint32 {
	return (ptr + alignment - 1) &^ (alignment - 1)
}

// isOption checks if a reflect.Type structurally matches our Option[T] generic.
func isOption(typ reflect.Type) bool {
	return strings.HasPrefix(typ.String(), "witgo.Option")
	// return typ.Kind() == reflect.Struct &&
	// 	typ.NumField() == 2 &&
	// 	typ.Field(0).Name == "HasValue" &&
	// 	typ.Field(0).Type.Kind() == reflect.Bool
}

// isResult checks if a reflect.Type structurally matches our Result[T, E] generic.
func isResult(typ reflect.Type) bool {
	return strings.HasPrefix(typ.String(), "witgo.Result")
	// return typ.Kind() == reflect.Struct &&
	// 	typ.NumField() == 3 &&
	// 	typ.Field(0).Name == "IsErr" &&
	// 	typ.Field(0).Type.Kind() == reflect.Bool &&
	// 	typ.Field(1).Name == "Ok" &&
	// 	typ.Field(2).Name == "Err"
}

// isVariant checks if a struct has fields with the `wit` tag.
func isVariant(typ reflect.Type) bool {
	if typ.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < typ.NumField(); i++ {
		if _, ok := typ.Field(i).Tag.Lookup("wit"); ok {
			return true
		}
	}
	return false
}

type Flagger interface {
	IsFlags()
}

var flaggerType = reflect.TypeFor[Flagger]()

func isFlags(typ reflect.Type) bool {
	if typ.Kind() != reflect.Struct {
		return false
	}

	ptrType := reflect.PointerTo(typ)
	return ptrType.Implements(flaggerType)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
