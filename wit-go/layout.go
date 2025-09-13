package witgo

import (
	"fmt"
	"reflect"
	"sync"
)

// TypeLayout describes the memory layout of a WIT type.
type TypeLayout struct {
	Size      uint32
	Alignment uint32
	Fields    []FieldLayout // For structs
}

// FieldLayout describes a field within a struct.
type FieldLayout struct {
	StructField reflect.StructField
	Offset      uint32
	Layout      *TypeLayout
}

var layoutCache = sync.Map{}

// GetOrCalculateLayout is the entry point for obtaining a type's layout.
func GetOrCalculateLayout(typ reflect.Type) (*TypeLayout, error) {
	if layout, ok := layoutCache.Load(typ); ok {
		return layout.(*TypeLayout), nil
	}
	layout, err := calculateLayout(typ)
	if err != nil {
		return nil, err
	}
	layoutCache.Store(typ, layout)
	return layout, nil
}

func calculateLayout(typ reflect.Type) (*TypeLayout, error) {
	// Check for our special generic types first.
	// This is a simplified check; a real library might need more robust reflection.
	if typ.Name() == "Option" && typ.NumField() == 2 {
		return calculateSumLayout(typ, 0, []reflect.Type{typ.Field(1).Type})
	}
	if typ.Name() == "Result" && typ.NumField() == 3 {
		return calculateSumLayout(typ, 0, []reflect.Type{typ.Field(1).Type, typ.Field(2).Type})
	}

	// Check for tagged variant structs
	if isVariant(typ) {
		return calculateSumLayout(typ, 0, getVariantCaseTypes(typ))
	}

	switch typ.Kind() {
	case reflect.Uint8, reflect.Int8, reflect.Bool:
		return &TypeLayout{Size: 1, Alignment: 1}, nil
	case reflect.Uint16, reflect.Int16:
		return &TypeLayout{Size: 2, Alignment: 2}, nil
	case reflect.Uint32, reflect.Int32, reflect.Float32:
		return &TypeLayout{Size: 4, Alignment: 4}, nil
	case reflect.Uint64, reflect.Int64, reflect.Float64:
		return &TypeLayout{Size: 8, Alignment: 8}, nil
	case reflect.String, reflect.Slice:
		return &TypeLayout{Size: 8, Alignment: 4}, nil // {ptr, len}
	case reflect.Struct:
		return calculateStructLayout(typ)
	case reflect.Array:
		return calculateArrayLayout(typ)
	default:
		return nil, fmt.Errorf("unsupported type for layout calculation: %v", typ)
	}
}

// calculateSumLayout computes layout for "sum" types like option, result, variant, enum.
// `cases` are the potential payload types. For enums, it's empty.
func calculateSumLayout(typ reflect.Type, discOffset int, cases []reflect.Type) (*TypeLayout, error) {
	numCases := len(cases)
	if numCases == 0 { // Enum case
		numCases = typ.NumField()
	}

	var discSize uint32
	switch {
	case numCases <= 1<<8:
		discSize = 1
	case numCases <= 1<<16:
		discSize = 2
	default:
		discSize = 4
	}

	var maxCaseSize, maxCaseAlign uint32 = 0, 1
	for _, caseType := range cases {
		caseLayout, err := GetOrCalculateLayout(caseType)
		if err != nil {
			return nil, err
		}
		if caseLayout.Size > maxCaseSize {
			maxCaseSize = caseLayout.Size
		}
		if caseLayout.Alignment > maxCaseAlign {
			maxCaseAlign = caseLayout.Alignment
		}
	}

	payloadOffset := align(discSize, maxCaseAlign)
	totalSize := payloadOffset + maxCaseSize

	return &TypeLayout{
		Size:      align(totalSize, maxCaseAlign),
		Alignment: maxCaseAlign,
	}, nil
}

func getVariantCaseTypes(typ reflect.Type) []reflect.Type {
	var types []reflect.Type
	for i := 0; i < typ.NumField(); i++ {
		if _, ok := typ.Field(i).Tag.Lookup("wit"); ok {
			types = append(types, typ.Field(i).Type)
		}
	}
	return types
}

func calculateStructLayout(typ reflect.Type) (*TypeLayout, error) {
	var fields []FieldLayout
	var currentOffset uint32 = 0
	var maxAlignment uint32 = 1

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldLayout, err := GetOrCalculateLayout(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		currentOffset = align(currentOffset, fieldLayout.Alignment)
		fields = append(fields, FieldLayout{
			StructField: field,
			Offset:      currentOffset,
			Layout:      fieldLayout,
		})

		currentOffset += fieldLayout.Size
		if fieldLayout.Alignment > maxAlignment {
			maxAlignment = fieldLayout.Alignment
		}
	}

	return &TypeLayout{
		Size:      align(currentOffset, maxAlignment),
		Alignment: maxAlignment,
		Fields:    fields,
	}, nil
}

// calculateArrayLayout computes the layout for a Go array, which corresponds to a WIT tuple.
func calculateArrayLayout(typ reflect.Type) (*TypeLayout, error) {
	if typ.Len() == 0 {
		return &TypeLayout{Size: 0, Alignment: 1}, nil
	}
	elemLayout, err := GetOrCalculateLayout(typ.Elem())
	if err != nil {
		return nil, err
	}
	// In WIT, tuples (Go arrays) are laid out sequentially with padding.
	var currentOffset uint32 = 0
	for i := 0; i < typ.Len(); i++ {
		currentOffset = align(currentOffset, elemLayout.Alignment)
		currentOffset += elemLayout.Size
	}
	return &TypeLayout{
		Size:      align(currentOffset, elemLayout.Alignment),
		Alignment: elemLayout.Alignment,
	}, nil
}
