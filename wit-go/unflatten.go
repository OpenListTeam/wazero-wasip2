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
	outVal := reflect.New(targetType).Elem() // Create a new value to populate.

	if isVariant(targetType) {
		return h.unflattenVariant(ctx, mem, ps, targetType)
	}
	if isFlags(targetType) {
		return h.unflattenFlags(ctx, mem, ps, targetType)
	}

	switch targetType.Kind() {
	case reflect.String:
		return h.unflattenString(ctx, mem, ps, targetType)
	case reflect.Slice:
		return h.unflattenSlice(ctx, mem, ps, targetType)
	case reflect.Struct:
		return h.unflattenStruct(ctx, mem, ps, targetType)
	case reflect.Array:
		return h.unflattenArray(ctx, mem, ps, targetType)
	case reflect.Bool:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for bool")
		}
		outVal.SetBool(p != 0)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for %v", targetType)
		}
		outVal.SetInt(int64(p))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for %v", targetType)
		}
		outVal.SetUint(p)
	case reflect.Float32:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for float32")
		}
		outVal.SetFloat(float64(math.Float32frombits(uint32(p))))
	case reflect.Float64:
		p, ok := ps.Next()
		if !ok {
			return reflect.Value{}, fmt.Errorf("not enough params on stack for float64")
		}
		outVal.SetFloat(math.Float64frombits(p))
	default:
		return reflect.Value{}, fmt.Errorf("unsupported type for unflattening: %v", targetType.Kind())
	}

	return outVal, nil
}

func (h *Host) unflattenStruct(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	for i := 0; i < outVal.NumField(); i++ {
		fieldVal, err := h.unflattenParam(ctx, mem, ps, outVal.Field(i).Type())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("failed to unflatten field %s: %w", outVal.Type().Field(i).Name, err)
		}
		outVal.Field(i).Set(fieldVal)
	}
	return outVal, nil
}

func (h *Host) unflattenString(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	ptr, ok1 := ps.Next()
	length, ok2 := ps.Next()
	if !ok1 || !ok2 {
		return reflect.Value{}, fmt.Errorf("not enough params on stack for string")
	}
	s, err := LowerStringFromParts(mem, uint32(ptr), uint32(length))
	if err != nil {
		return reflect.Value{}, err
	}
	outVal.SetString(s)
	return outVal, nil
}

func (h *Host) unflattenSlice(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	ptr, ok1 := ps.Next()
	length, ok2 := ps.Next()
	if !ok1 || !ok2 {
		return reflect.Value{}, fmt.Errorf("not enough params on stack for slice")
	}

	// We must use `Lower` here, as it contains the generic logic for handling list<T>.
	// We do this by temporarily creating the {ptr, len} structure in memory.
	tempPtr, err := h.allocator.Allocate(ctx, 8, 4)
	if err != nil {
		return reflect.Value{}, err
	}
	if !mem.WriteUint32Le(tempPtr, uint32(ptr)) {
		return reflect.Value{}, fmt.Errorf("failed to write temp slice ptr")
	}
	if !mem.WriteUint32Le(tempPtr+4, uint32(length)) {
		return reflect.Value{}, fmt.Errorf("failed to write temp slice len")
	}

	err = Lower(ctx, mem, tempPtr, outVal)
	return outVal, err
}

func (h *Host) unflattenArray(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	for i := 0; i < outVal.Len(); i++ {
		elemVal, err := h.unflattenParam(ctx, mem, ps, outVal.Index(i).Type())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("failed to unflatten array element %d: %w", i, err)
		}
		outVal.Index(i).Set(elemVal)
	}
	return outVal, nil
}

func (h *Host) unflattenFlags(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	p, ok := ps.Next()
	if !ok {
		return reflect.Value{}, fmt.Errorf("not enough params on stack for flags %v", targetType)
	}

	for i := 0; i < outVal.NumField(); i++ {
		if (p & (1 << i)) != 0 {
			outVal.Field(i).SetBool(true)
		} else {
			outVal.Field(i).SetBool(false)
		}
	}
	return outVal, nil
}

func (h *Host) unflattenVariant(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType).Elem()
	disc, ok := ps.Next()
	if !ok {
		return reflect.Value{}, fmt.Errorf("not enough params on stack for variant discriminant")
	}

	// Determine shapes to correctly consume padding
	numCases := targetType.NumField()
	payloadShapes := make([][]uint64, numCases)
	for i := 0; i < numCases; i++ {
		field := targetType.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		var shape []uint64
		if err := h.flattenParam(ctx, reflect.Zero(fieldType), &shape); err != nil {
			return reflect.Value{}, err
		}
		payloadShapes[i] = shape
	}

	for i := 0; i < numCases; i++ {
		field := targetType.Field(i)
		if uint64(i) == disc {
			payloadField := outVal.FieldByName(field.Name)
			if payloadField.Kind() == reflect.Pointer {
				payloadField.Set(reflect.New(payloadField.Type().Elem()))
				payloadField = payloadField.Elem()
			}
			val, err := h.unflattenParam(ctx, mem, ps, payloadField.Type())
			if err != nil {
				return reflect.Value{}, err
			}
			payloadField.Set(val)
		} else {
			// Consume padding
			for j := 0; j < len(payloadShapes[i]); j++ {
				ps.Next()
			}
		}
	}
	return outVal, nil
}

// makeWrapperFunc creates a dynamic function using reflect.MakeFunc.
func (e *Exporter) makeWrapperFunc(funcType reflect.Type, funcVal reflect.Value) (interface{}, error) {
	flatIn, hasRetptr, err := e.flattenSignatureTypes(funcType)
	if err != nil {
		return nil, err
	}

	// For host exports, the wasm function signature always returns void.
	var flatOut []reflect.Type

	wrapperIn := append([]reflect.Type{
		reflect.TypeFor[context.Context](),
		reflect.TypeFor[api.Module](),
	}, flatIn...)

	wrapperType := reflect.FuncOf(wrapperIn, flatOut, false)

	wrapperImpl := func(args []reflect.Value) []reflect.Value {
		ctx := args[0].Interface().(context.Context)
		module := args[1].Interface().(api.Module)

		h, err := e.getHost(module)
		if err != nil {
			panic(fmt.Sprintf("failed to get host for calling module: %v", err))
		}

		var retptr uint32
		numFlatParams := len(args) - 2
		if hasRetptr {
			if numFlatParams == 0 {
				panic("function expected a return pointer, but received no parameters")
			}
			// The retptr is always the LAST wasm parameter.
			retptr = uint32(args[len(args)-1].Uint())
			numFlatParams-- // Don't consume the retptr as a normal parameter.
		}

		paramStream := &paramStream{params: make([]uint64, 0, numFlatParams)}
		for _, arg := range args[2 : 2+numFlatParams] {
			paramStream.params = append(paramStream.params, arg.Uint())
		}

		callArgs := make([]reflect.Value, funcType.NumIn())
		for i := 0; i < funcType.NumIn(); i++ {
			paramType := funcType.In(i)
			val, err := h.unflattenParam(ctx, module.Memory(), paramStream, paramType)
			if err != nil {
				panic(fmt.Sprintf("failed to unflatten parameter %d for %s: %v", i, funcVal.Type().Name(), err))
			}
			callArgs[i] = val
		}

		results := funcVal.Call(callArgs)

		if hasRetptr {
			if len(results) == 0 {
				panic("function was expected to return a value but did not")
			}
			// Lift the complex result into the pointer provided by the guest.
			err := LiftToPtr(ctx, module.Memory(), h.allocator, results[0], retptr)
			if err != nil {
				panic(fmt.Sprintf("failed to lift result to retptr: %v", err))
			}
		}
		// The wasm function is void, so we always return nil.
		return nil
	}

	return reflect.MakeFunc(wrapperType, wrapperImpl).Interface(), nil
}

// flattenSignatureTypes calculates the flattened Go reflect.Type slice for a function's parameters.
// It also returns a boolean indicating if a return pointer is needed.
func (e *Exporter) flattenSignatureTypes(funcType reflect.Type) ([]reflect.Type, bool, error) {
	var flatTypes []reflect.Type

	// Flatten all input parameters.
	for i := 0; i < funcType.NumIn(); i++ {
		typ := funcType.In(i)
		var flatShape []uint64
		err := e.dummyHost.flattenParam(context.Background(), reflect.Zero(typ), &flatShape)
		if err != nil {
			return nil, false, fmt.Errorf("could not get shape for type %v: %w", typ, err)
		}
		for range flatShape {
			flatTypes = append(flatTypes, reflect.TypeFor[uint32]())
		}
	}

	// If the function has any return values, add a retptr to the parameter list.
	if funcType.NumOut() > 0 {
		flatTypes = append(flatTypes, reflect.TypeFor[uint32]())
		return flatTypes, true, nil
	}

	return flatTypes, false, nil
}
