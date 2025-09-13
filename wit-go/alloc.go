package witgo

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// GuestAllocator manages memory allocation within the Wasm guest.
// It requires the guest to export `cabi_realloc`.
type GuestAllocator struct {
	realloc api.Function
}

// NewGuestAllocator creates a new allocator by finding the `cabi_realloc` export.
func NewGuestAllocator(module api.Module) (*GuestAllocator, error) {
	reallocFunc := module.ExportedFunction("cabi_realloc")
	if reallocFunc == nil {
		return nil, fmt.Errorf("guest module must export `cabi_realloc` function")
	}
	return &GuestAllocator{realloc: reallocFunc}, nil
}

// Allocate reserves a block of memory in the guest.
func (a *GuestAllocator) Allocate(ctx context.Context, size, alignment uint32) (uint32, error) {
	// cabi_realloc(0, 0, alignment, size) is the call for a new allocation.
	results, err := a.realloc.Call(ctx, 0, 0, uint64(alignment), uint64(size))
	if err != nil {
		return 0, fmt.Errorf("cabi_realloc for allocate failed: %w", err)
	}
	return uint32(results[0]), nil
}

// Free releases a block of memory in the guest.
func (a *GuestAllocator) Free(ctx context.Context, ptr, size, alignment uint32) error {
	// cabi_realloc(ptr, size, alignment, 0) is the call for freeing memory.
	_, err := a.realloc.Call(ctx, uint64(ptr), uint64(size), uint64(alignment), 0)
	if err != nil {
		return fmt.Errorf("cabi_realloc for free failed: %w", err)
	}
	return nil
}
