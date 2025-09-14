package witgo

import (
	"context"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Export registers a Go function as a host import for the guest module.
// It uses reflection to create a wrapper that automatically handles the
// ABI translation between Go types and the flattened Wasm representation.
func Export(builder wazero.HostModuleBuilder, funcName string, goFunc interface{}) error {
	funcVal := reflect.ValueOf(goFunc)
	funcType := funcVal.Type()

	if funcType.Kind() != reflect.Func {
		return fmt.Errorf("`goFunc` must be a function, but got %T", goFunc)
	}

	// This is the implementation of our wrapper function.
	wrapperImpl := func(args []reflect.Value) []reflect.Value {
		// The first argument from wazero is always context.Context.
		ctx := args[0].Interface().(context.Context)
		// The second is the calling module instance, which IS the guest.
		module := args[1].Interface().(api.Module)

		// Create a temporary host and param stream for this call.
		h, err := NewHost(module) // NewHost will create the allocator for us.
		if err != nil {
			panic(fmt.Sprintf("failed to create temporary host for export: %v", err))
		}
		paramStream := &paramStream{params: make([]uint64, 0, len(args)-2)}
		for _, arg := range args[2:] {
			paramStream.params = append(paramStream.params, arg.Uint())
		}

		// Unflatten parameters from the stream into Go values.
		callArgs := make([]reflect.Value, funcType.NumIn())
		for i := 0; i < funcType.NumIn(); i++ {
			paramType := funcType.In(i)
			val, err := h.unflattenParam(ctx, module.Memory(), paramStream, paramType)
			if err != nil {
				panic(fmt.Sprintf("failed to unflatten parameter %d for %s: %v", i, funcName, err))
			}
			callArgs[i] = val
		}

		// Call the user's high-level Go function.
		results := funcVal.Call(callArgs)

		if len(results) == 0 {
			return nil
		}

		// Lift the results back into Wasm memory/values.
		flatResults := make([]uint64, 0)

		for _, res := range results {
			// We flatten the result to get its ABI representation.
			err := h.flattenParam(ctx, res, &flatResults)
			if err != nil {
				panic(fmt.Sprintf("failed to flatten result: %v", err))
			}
		}

		// Convert the flattened uint64 results back to reflect.Value for the return.
		returnVals := make([]reflect.Value, len(flatResults))
		for i, res := range flatResults {
			// For wasm32, all flattened values are passed as i32.
			// wazero's Go function ABI expects this mapping.
			returnVals[i] = reflect.ValueOf(uint32(res))
		}
		return returnVals
	}

	// Dynamically create the wrapper function with the correct flat signature.
	wrapperFunc, err := makeWrapperFunc(funcType, wrapperImpl)
	if err != nil {
		return fmt.Errorf("failed to create wrapper for %s: %w", funcName, err)
	}

	builder.NewFunctionBuilder().WithFunc(wrapperFunc).Export(funcName)
	return nil
}

// makeWrapperFunc creates a dynamic function using reflect.MakeFunc.
func makeWrapperFunc(funcType reflect.Type, wrapperImpl func([]reflect.Value) []reflect.Value) (interface{}, error) {
	flatIn, err := flattenSignatureTypes(funcType, true)
	if err != nil {
		return nil, err
	}
	flatOut, err := flattenSignatureTypes(funcType, false)
	if err != nil {
		return nil, err
	}

	wrapperIn := append([]reflect.Type{
		reflect.TypeOf((*context.Context)(nil)).Elem(),
		reflect.TypeOf((*api.Module)(nil)).Elem(),
	}, flatIn...)

	wrapperType := reflect.FuncOf(wrapperIn, flatOut, false)
	return reflect.MakeFunc(wrapperType, wrapperImpl).Interface(), nil
}

// flattenSignatureTypes calculates the flattened Go reflect.Type slice for a function's parameters or results.
func flattenSignatureTypes(funcType reflect.Type, isInput bool) ([]reflect.Type, error) {
	var flatTypes []reflect.Type
	count := funcType.NumIn()
	if !isInput {
		count = funcType.NumOut()
	}

	dummyHost := &Host{} // For shape calculation, we don't need a real host.

	for i := 0; i < count; i++ {
		var typ reflect.Type
		if isInput {
			typ = funcType.In(i)
		} else {
			typ = funcType.Out(i)
		}

		var flatShape []uint64
		// We use a dummy flatten call to determine the number of parameters.
		err := dummyHost.flattenParam(context.Background(), reflect.Zero(typ), &flatShape) // Arena is nil for shape calculation
		if err != nil {
			return nil, fmt.Errorf("could not get shape for type %v: %w", typ, err)
		}

		for range flatShape {
			flatTypes = append(flatTypes, reflect.TypeOf(uint32(0)))
		}
	}
	return flatTypes, nil
}
