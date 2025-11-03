package witgo

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// unflattenParam is the inverse of flattenParam. It reconstructs a high-level Go value
// by consuming one or more flat values from a stream.
func (h *Host) unflattenParam(ctx context.Context, mem api.Memory, ps *paramStream, targetType reflect.Type) (reflect.Value, error) {
	outVal := reflect.New(targetType) // Create a new value to populate.
	for outVal.Kind() == reflect.Pointer {
		outVal = outVal.Elem()
	}

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

	err := lowerSlice2(ctx, mem, uint32(ptr), uint32(length), outVal)
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

// makeWrapperFunc creates a dynamic function using reflect.MakeFunc that can be
// exported to a Wasm module.
func (e *Exporter) makeWrapperFunc(funcType reflect.Type, funcVal reflect.Value) (interface{}, error) {
	flatIn, flatOut, hasRetptr, err := e.flattenSignatureTypes(funcType)
	if err != nil {
		return nil, err
	}

	wrapperIn := append([]reflect.Type{
		reflect.TypeFor[context.Context](),
		reflect.TypeFor[api.Module](),
	}, flatIn...)

	wrapperType := reflect.FuncOf(wrapperIn, flatOut, false)

	wrapperImpl := func(args []reflect.Value) []reflect.Value {
		ctx := args[0].Interface().(context.Context)
		module := args[1].Interface().(api.Module)

		h, err := NewHost(module)
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
			switch arg.Kind() {
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				paramStream.params = append(paramStream.params, uint64(arg.Int()))
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				paramStream.params = append(paramStream.params, arg.Uint())
			case reflect.Float32:
				paramStream.params = append(paramStream.params, uint64(math.Float32bits(float32(arg.Float()))))
			case reflect.Float64:
				paramStream.params = append(paramStream.params, uint64(math.Float64bits(arg.Float())))
			}
		}

		callArgs := make([]reflect.Value, funcType.NumIn())
		funcParamIndex := 0

		if len(callArgs) > 0 && funcType.In(0) == reflect.TypeFor[context.Context]() {
			callArgs[0] = args[0]
			funcParamIndex = 1
		}

		for ; funcParamIndex < len(callArgs); funcParamIndex++ {
			paramType := funcType.In(funcParamIndex)
			val, err := h.unflattenParam(ctx, module.Memory(), paramStream, paramType)
			if err != nil {
				panic(fmt.Sprintf("failed to unflatten parameter %d for %s: %v", funcParamIndex, funcVal.Type().Name(), err))
			}
			callArgs[funcParamIndex] = val
		}

		results := funcVal.Call(callArgs)

		// Handle return values
		if hasRetptr {
			// Complex type: lift the result to the guest-provided pointer.
			if len(results) == 0 {
				panic("function was expected to return a value but did not")
			}
			err := LiftToPtr(ctx, module.Memory(), h.allocator, results[0], retptr)
			if err != nil {
				panic(fmt.Sprintf("failed to lift result to retptr: %v", err))
			}
			return nil // The wasm function is void.
		} else if len(flatOut) > 0 {
			// Scalar type: return the value directly.
			if len(results) == 0 {
				panic("function was expected to return a scalar value but did not")
			}
			// Convert the Go scalar into a Wasm-compatible scalar and return it.
			outVal := results[0]
			ret := reflect.New(flatOut[0]).Elem()
			switch outVal.Kind() {
			case reflect.Bool:
				if outVal.Bool() {
					ret.SetUint(1)
				} else {
					ret.SetUint(0)
				}
			case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				ret.SetInt(int64(outVal.Int()))
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				ret.SetUint(outVal.Uint())
			case reflect.Float32, reflect.Float64:
				ret.SetFloat(outVal.Float())
			}
			return []reflect.Value{ret}
		}

		return nil // No return values.
	}

	return reflect.MakeFunc(wrapperType, wrapperImpl).Interface(), nil
}

// flattenSignatureTypes gets the wasm signature for a go func, correctly applying all ABI rules for returns.
func (e *Exporter) flattenSignatureTypes(funcType reflect.Type) (inTypes, outTypes []reflect.Type, isIndirectReturn bool, err error) {
	// 1. Flatten input types.
	for i := 0; i < funcType.NumIn(); i++ {
		typ := funcType.In(i)
		if i == 0 && typ == reflect.TypeFor[context.Context]() {
			continue
		}
		flatTypes, err := e.flattenType(typ)
		if err != nil {
			return nil, nil, false, fmt.Errorf("could not get shape for input type %v: %w", typ, err)
		}
		inTypes = append(inTypes, flatTypes...)
	}

	// 2. Decide if an indirect return is necessary, following a priority of ABI rules.
	useIndirectReturn := false

	// Rule 1 (Highest Priority): Intrinsically complex types like string/slice must be indirect.
	for i := 0; i < funcType.NumOut(); i++ {
		typ := funcType.Out(i)
		kind := typ.Kind()
		if kind == reflect.Ptr {
			kind = typ.Elem().Kind()
		}
		if kind == reflect.String || kind == reflect.Slice {
			useIndirectReturn = true
			break
		}
	}

	// Rule 2 (Next Priority): Records/variants are indirect if they flatten to more than one value.
	if !useIndirectReturn {
		for i := 0; i < funcType.NumOut(); i++ {
			typ := funcType.Out(i)
			kind := typ.Kind()
			if kind == reflect.Ptr {
				kind = typ.Elem().Kind()
			}

			if kind == reflect.Struct || kind == reflect.Array || isVariant(typ) {
				// Flatten this type individually to check its size.
				individualFlatTypes, err := e.flattenType(typ)
				if err != nil {
					return nil, nil, false, fmt.Errorf("could not get shape for individual output type %v: %w", typ, err)
				}
				if len(individualFlatTypes) > 1 {
					useIndirectReturn = true
					break
				}
			}
		}
	}

	// Flatten all return types together to check total size for the final rule.
	var initialOutTypes []reflect.Type
	for i := 0; i < funcType.NumOut(); i++ {
		typ := funcType.Out(i)
		flatTypes, err := e.flattenType(typ)
		if err != nil {
			return nil, nil, false, fmt.Errorf("could not get shape for output type %v: %w", typ, err)
		}
		initialOutTypes = append(initialOutTypes, flatTypes...)
	}

	// Rule 3 (Lowest Priority): If not complex by other rules, check if total size is too large for registers.
	if !useIndirectReturn && len(initialOutTypes) > 2 {
		useIndirectReturn = true
	}

	// 3. Construct final signatures based on the decision.
	if useIndirectReturn {
		isIndirectReturn = true
		inTypes = append(inTypes, reflect.TypeFor[uint32]()) // Add pointer param
		outTypes = []reflect.Type{}                          // Void return
	} else {
		isIndirectReturn = false
		outTypes = initialOutTypes // Direct return
	}

	return
}

// flattenType recursively deconstructs a Go type, returning the flat Wasm parameter types.
func (e *Exporter) flattenType(typ reflect.Type) ([]reflect.Type, error) {
	if isVariant(typ) {
		if typ.Kind() != reflect.Struct {
			return nil, fmt.Errorf("variant type %v must be a struct", typ)
		}

		var casePayloads [][]reflect.Type

		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" { // Skip unexported fields
				continue
			}

			var flatPayload []reflect.Type
			var err error
			payloadType := field.Type

			isUnit := false
			if payloadType.Kind() == reflect.Ptr {
				elem := payloadType.Elem()
				if elem.Name() == "Unit" || (elem.Kind() == reflect.Struct && elem.NumField() == 0) {
					isUnit = true
				}
			}

			if isUnit {
				flatPayload = []reflect.Type{}
			} else {
				flatPayload, err = e.flattenType(payloadType)
				if err != nil {
					return nil, fmt.Errorf("failed to flatten variant case %s: %w", field.Name, err)
				}
			}
			casePayloads = append(casePayloads, flatPayload)
		}

		maxPayload := maxFlat(casePayloads...)

		return append([]reflect.Type{reflect.TypeFor[uint32]()}, maxPayload...), nil
	}

	if isFlags(typ) {
		return []reflect.Type{reflect.TypeFor[uint32]()}, nil
	}

	switch typ.Kind() {
	case reflect.String, reflect.Slice:
		return []reflect.Type{reflect.TypeFor[uint32](), reflect.TypeFor[uint32]()}, nil
	case reflect.Struct:
		var flatTypes []reflect.Type
		for i := 0; i < typ.NumField(); i++ {
			fieldTypes, err := e.flattenType(typ.Field(i).Type)
			if err != nil {
				return nil, err
			}
			flatTypes = append(flatTypes, fieldTypes...)
		}
		return flatTypes, nil
	case reflect.Array:
		var flatTypes []reflect.Type
		elemType := typ.Elem()
		for i := 0; i < typ.Len(); i++ {
			elemTypes, err := e.flattenType(elemType)
			if err != nil {
				return nil, err
			}
			flatTypes = append(flatTypes, elemTypes...)
		}
		return flatTypes, nil
	case reflect.Ptr:
		return e.flattenType(typ.Elem())
	case reflect.Bool:
		return []reflect.Type{reflect.TypeFor[uint32]()}, nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return []reflect.Type{reflect.TypeFor[uint32]()}, nil
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return []reflect.Type{reflect.TypeFor[int32]()}, nil
	case reflect.Uint64:
		return []reflect.Type{reflect.TypeFor[uint64]()}, nil
	case reflect.Int64:
		return []reflect.Type{reflect.TypeFor[int64]()}, nil
	case reflect.Float32:
		return []reflect.Type{reflect.TypeFor[float32]()}, nil
	case reflect.Float64:
		return []reflect.Type{reflect.TypeFor[float64]()}, nil
	default:
		return nil, fmt.Errorf("unsupported parameter kind for flattening: %v", typ.Kind())
	}
}
