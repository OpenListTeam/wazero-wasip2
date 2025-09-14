package witgo

import (
	"reflect"
)

// align aligns ptr to the given alignment.
func align(ptr, alignment uint32) uint32 {
	return (ptr + alignment - 1) &^ (alignment - 1)
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
