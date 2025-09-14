package witgo

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Exporter provides a convenient, chainable API for exporting Go functions to a guest.
// It caches reflection-based wrappers for performance.
type Exporter struct {
	wazero.HostModuleBuilder
	hostCache map[api.Module]*Host
	mu        sync.Mutex
	dummyHost *Host // For shape calculation

	// Cache for generated wrapper functions, mapping Go func type to the wrapper.
	wrapperCache map[reflect.Type]interface{}
}

// NewExporter creates a new Exporter that wraps a wazero.HostModuleBuilder.
func NewExporter(builder wazero.HostModuleBuilder) *Exporter {
	return &Exporter{
		HostModuleBuilder: builder,
		hostCache:         make(map[api.Module]*Host),
		dummyHost:         &Host{},
		wrapperCache:      make(map[reflect.Type]interface{}),
	}
}

// MustExport is a convenience wrapper around Export that panics on error.
func (e *Exporter) MustExport(funcName string, goFunc interface{}) *Exporter {
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

	// --- CACHING LOGIC START ---
	e.mu.Lock()
	wrapperFunc, found := e.wrapperCache[funcType]
	if !found {
		var err error
		wrapperFunc, err = e.makeWrapperFunc(funcType, funcVal)
		if err != nil {
			e.mu.Unlock()
			return fmt.Errorf("failed to create wrapper for %s: %w", funcName, err)
		}
		e.wrapperCache[funcType] = wrapperFunc
	}
	e.mu.Unlock()
	// --- CACHING LOGIC END ---

	e.NewFunctionBuilder().WithFunc(wrapperFunc).Export(funcName)
	return nil
}

// getHost is a helper on the Exporter to manage Host instances for guest callers.
func (e *Exporter) getHost(module api.Module) (*Host, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if host, ok := e.hostCache[module]; ok {
		return host, nil
	}
	host, err := NewHost(module)
	if err != nil {
		return nil, err
	}
	e.hostCache[module] = host
	return host, nil
}
