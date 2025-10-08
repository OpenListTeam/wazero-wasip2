package witgo

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// lifter is the interface for a type-specific, pre-compiled "lifting" function.
type lifter interface {
	lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error)
}

// lowerer is the interface for a type-specific, pre-compiled "lowering" function.
type lowerer interface {
	lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error
}

var (
	lifterCache  sync.Map
	lowererCache sync.Map
)

// getOrGenerateLifter is the main entry point for the codec cache.
// It returns a cached or newly generated lifter for the given type.
func getOrGenerateLifter(typ reflect.Type) (lifter, error) {
	if l, ok := lifterCache.Load(typ); ok {
		return l.(lifter), nil
	}
	l, err := generateLifter(typ)
	if err != nil {
		return nil, err
	}
	lifterCache.Store(typ, l)
	return l, nil
}

// getOrGenerateLowerer does the same for lowerers.
func getOrGenerateLowerer(typ reflect.Type) (lowerer, error) {
	if l, ok := lowererCache.Load(typ); ok {
		return l.(lowerer), nil
	}
	l, err := generateLowerer(typ)
	if err != nil {
		return nil, err
	}
	lowererCache.Store(typ, l)
	return l, nil
}

// generateLifter is the recursive function that builds the lifter codec.
func generateLifter(typ reflect.Type) (lifter, error) {
	switch {
	case isVariant(typ):
		return newVariantLifter(typ)
	case isFlags(typ):
		return newFlagsLifter(), nil
	}

	switch typ.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16,
		reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return newPrimitiveLifter(), nil
	case reflect.String:
		return &stringLifter{}, nil
	case reflect.Slice:
		return newSliceLifter(typ)
	case reflect.Struct:
		return newStructLifter(typ)
	case reflect.Array:
		return newArrayLifter(typ)
	case reflect.Pointer:
		return newPointerLifter(typ)
	default:
		return nil, fmt.Errorf("unsupported type for lifter generation: %v", typ)
	}
}

// generateLowerer is the recursive function that builds the lowerer codec.
func generateLowerer(typ reflect.Type) (lowerer, error) {
	layout, err := GetOrCalculateLayout(typ)
	if err != nil {
		return nil, err
	}

	switch {
	case isVariant(typ):
		return newVariantLowerer(typ)
	case isFlags(typ):
		return newFlagsLowerer(), nil
	}

	switch typ.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16,
		reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return newPrimitiveLowerer(), nil
	case reflect.String:
		return &stringLowerer{}, nil
	case reflect.Slice:
		return newSliceLowerer(typ)
	case reflect.Struct:
		return newStructLowerer(typ, layout) // FIX: Pass layout here
	case reflect.Array:
		return newArrayLowerer(typ)
	case reflect.Pointer:
		return newPointerLowerer(typ)
	default:
		return nil, fmt.Errorf("unsupported type for lowerer generation: %v", typ)
	}
}

// --- Concrete Lifter Implementations ---

type primitiveLifter struct{}

func newPrimitiveLifter() *primitiveLifter { return &primitiveLifter{} }
func (l *primitiveLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := write(ctx, h.module.Memory(), h.allocator, val, ptr, layout); err != nil {
		return 0, err
	}
	return ptr, nil
}

type stringLifter struct{}

func (l *stringLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := liftString(ctx, h.module.Memory(), h.allocator, val.String(), ptr); err != nil {
		return 0, err
	}
	return ptr, nil
}

type structLifter struct {
	fieldCodecs []lifter
	fieldNames  []string
}

func newStructLifter(typ reflect.Type) (*structLifter, error) {
	codecs := make([]lifter, typ.NumField())
	names := make([]string, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		var err error
		field := typ.Field(i)
		codecs[i], err = getOrGenerateLifter(field.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to generate lifter for field %s: %w", field.Name, err)
		}
		names[i] = field.Name
	}
	return &structLifter{fieldCodecs: codecs, fieldNames: names}, nil
}
func (l *structLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := liftStruct(ctx, h.module.Memory(), h.allocator, val, ptr, layout); err != nil {
		return 0, err
	}
	return ptr, nil
}

type sliceLifter struct {
	elemLifter lifter
}

func newSliceLifter(typ reflect.Type) (*sliceLifter, error) {
	lifter, err := getOrGenerateLifter(typ.Elem())
	if err != nil {
		return nil, err
	}
	return &sliceLifter{elemLifter: lifter}, nil
}
func (l *sliceLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := liftSlice(ctx, h.module.Memory(), h.allocator, val, ptr); err != nil {
		return 0, err
	}
	return ptr, nil
}

type arrayLifter struct {
	elemLifter lifter
}

func newArrayLifter(typ reflect.Type) (*arrayLifter, error) {
	lifter, err := getOrGenerateLifter(typ.Elem())
	if err != nil {
		return nil, err
	}
	return &arrayLifter{elemLifter: lifter}, nil
}
func (l *arrayLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := liftArray(ctx, h.module.Memory(), h.allocator, val, ptr); err != nil {
		return 0, err
	}
	return ptr, nil
}

type pointerLifter struct {
	elemLifter lifter
}

func newPointerLifter(typ reflect.Type) (*pointerLifter, error) {
	lifter, err := getOrGenerateLifter(typ.Elem())
	if err != nil {
		return nil, err
	}
	return &pointerLifter{elemLifter: lifter}, nil
}
func (l *pointerLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	return l.elemLifter.lift(ctx, h, val.Elem(), layout)
}

type flagsLifter struct{}

func newFlagsLifter() *flagsLifter { return &flagsLifter{} }
func (l *flagsLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := write(ctx, h.module.Memory(), h.allocator, val, ptr, layout); err != nil {
		return 0, err
	}
	return ptr, nil
}

type variantLifter struct {
	caseLifters []lifter
}

func newVariantLifter(typ reflect.Type) (*variantLifter, error) {
	lifters := make([]lifter, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		var err error
		field := typ.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		lifters[i], err = getOrGenerateLifter(fieldType)
		if err != nil {
			return nil, err
		}
	}
	return &variantLifter{caseLifters: lifters}, nil
}
func (l *variantLifter) lift(ctx context.Context, h *Host, val reflect.Value, layout *TypeLayout) (uint32, error) {
	ptr, err := h.allocator.Allocate(ctx, layout.Size, layout.Alignment)
	if err != nil {
		return 0, err
	}
	if err := write(ctx, h.module.Memory(), h.allocator, val, ptr, layout); err != nil {
		return 0, err
	}
	return ptr, nil
}

// --- Concrete Lowerer Implementations ---

type primitiveLowerer struct{}

func newPrimitiveLowerer() *primitiveLowerer { return &primitiveLowerer{} }
func (l *primitiveLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	layout, err := GetOrCalculateLayout(outVal.Type())
	if err != nil {
		return err
	}
	return read(ctx, h.module.Memory(), ptr, outVal, layout)
}

type stringLowerer struct{}

func (l *stringLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	s, err := lowerString(h.module.Memory(), ptr)
	if err != nil {
		return err
	}
	outVal.SetString(s)
	return nil
}

type structLowerer struct {
	fieldCodecs []lowerer
	fieldNames  []string
}

func newStructLowerer(typ reflect.Type, layout *TypeLayout) (*structLowerer, error) {
	return &structLowerer{}, nil
}
func (l *structLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	layout, err := GetOrCalculateLayout(outVal.Type())
	if err != nil {
		return err
	}
	return lowerStruct(ctx, h.module.Memory(), ptr, outVal, layout)
}

type sliceLowerer struct {
	elemLowerer lowerer
}

func newSliceLowerer(typ reflect.Type) (*sliceLowerer, error) {
	return &sliceLowerer{}, nil
}
func (l *sliceLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	return lowerSlice(ctx, h.module.Memory(), ptr, outVal)
}

type arrayLowerer struct {
	elemLowerer lowerer
}

func newArrayLowerer(typ reflect.Type) (*arrayLowerer, error) {
	return &arrayLowerer{}, nil
}
func (l *arrayLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	return lowerArray(ctx, h.module.Memory(), ptr, outVal)
}

type pointerLowerer struct {
	elemLowerer lowerer
}

func newPointerLowerer(typ reflect.Type) (*pointerLowerer, error) {
	return &pointerLowerer{}, nil
}
func (l *pointerLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	return nil
}

type flagsLowerer struct{}

func newFlagsLowerer() *flagsLowerer { return &flagsLowerer{} }
func (l *flagsLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	layout, err := GetOrCalculateLayout(outVal.Type())
	if err != nil {
		return err
	}
	return read(ctx, h.module.Memory(), ptr, outVal, layout)
}

type variantLowerer struct {
	caseLowerers []lowerer
}

func newVariantLowerer(typ reflect.Type) (*variantLowerer, error) {
	return &variantLowerer{}, nil
}
func (l *variantLowerer) lower(ctx context.Context, h *Host, ptr uint32, outVal reflect.Value) error {
	layout, err := GetOrCalculateLayout(outVal.Type())
	if err != nil {
		return err
	}
	return read(ctx, h.module.Memory(), ptr, outVal, layout)
}
