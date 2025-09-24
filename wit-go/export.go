package witgo

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/tetratelabs/wazero"
)

// Exporter provides a convenient, chainable API for exporting Go functions to a guest.
// It caches reflection-based wrappers for performance.
type Exporter struct {
	wazero.HostModuleBuilder
	mu sync.Mutex

	// Cache for generated wrapper functions, mapping Go func type to the wrapper.
	wrapperCache map[uintptr]any
}

// NewExporter creates a new Exporter that wraps a wazero.HostModuleBuilder.
func NewExporter(builder wazero.HostModuleBuilder) *Exporter {
	return &Exporter{
		HostModuleBuilder: builder,
		wrapperCache:      make(map[uintptr]any),
	}
}

// MustExport is a convenience wrapper around Export that panics on error.
func (e *Exporter) MustExport(funcName string, goFunc any) *Exporter {
	if err := e.Export(funcName, goFunc); err != nil {
		panic(err)
	}
	return e
}

// Export registers a Go function as a host import for the guest module.
func (e *Exporter) Export(funcName string, goFunc interface{}) error {
	funcVal := reflect.ValueOf(goFunc)
	funcType := funcVal.Type()

	if funcType.Kind() != reflect.Func {
		return fmt.Errorf("`goFunc` must be a function, but got %T", goFunc)
	}

	// 使用函数指针作为缓存的唯一键
	funcPtr := funcVal.Pointer()

	e.mu.Lock()
	wrapperFunc, found := e.wrapperCache[funcPtr]
	if !found {
		var err error
		wrapperFunc, err = e.makeWrapperFunc(funcType, funcVal)
		if err != nil {
			e.mu.Unlock()
			return fmt.Errorf("failed to create wrapper for %s: %w", funcName, err)
		}
		e.wrapperCache[funcPtr] = wrapperFunc
	}
	e.mu.Unlock()

	e.NewFunctionBuilder().WithFunc(wrapperFunc).Export(funcName)
	return nil
}
