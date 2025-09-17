package witgo

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// allocatorMode 用于定义检测到的 Guest 内存管理模式
type allocatorMode int

const (
	modeUnsupported allocatorMode = iota
	modeCabiRealloc               // 标准组件模型: cabi_realloc(ptr, old_size, align, new_size)
	modeRealloc                   // TinyGo 或 C-style: realloc(ptr, new_size)
	modeMallocFree                // C-style: malloc(size), free(ptr)
)

// Allocator 能够智能管理不同 Wasm Guest 的内存。
// 它会自动检测并适配多种内存管理接口。
type GuestAllocator struct {
	mode    allocatorMode
	realloc api.Function
	malloc  api.Function
	free    api.Function
}

// NewAllocator 通过检查模块导出的函数，创建一个新的、自适应的分配器。
// 它会按 `cabi_realloc` -> `realloc` -> `malloc`/`free` 的优先级进行回退。
func NewGuestAllocator(module api.Module) (*GuestAllocator, error) {
	// 优先级 1: 检查是否为标准的组件模型 `cabi_realloc`
	if reallocFunc := module.ExportedFunction("cabi_realloc"); reallocFunc != nil {
		return &GuestAllocator{
			mode:    modeCabiRealloc,
			realloc: reallocFunc,
		}, nil
	}

	// 优先级 2: 检查是否为 TinyGo 或 C-style 的 `realloc`
	if reallocFunc := module.ExportedFunction("realloc"); reallocFunc != nil {
		// 如果同时存在 `free`，则认为是 C-style realloc；否则认为是 TinyGo (GC)
		freeFunc := module.ExportedFunction("free")
		return &GuestAllocator{
			mode:    modeRealloc,
			realloc: reallocFunc,
			free:    freeFunc, // 如果 freeFunc 为 nil，Free() 方法将成为空操作
		}, nil
	}

	// 优先级 3: 检查是否为 C-style 的 `malloc` 和 `free`
	if mallocFunc := module.ExportedFunction("malloc"); mallocFunc != nil {
		if freeFunc := module.ExportedFunction("free"); freeFunc != nil {
			return &GuestAllocator{
				mode:   modeMallocFree,
				malloc: mallocFunc,
				free:   freeFunc,
			}, nil
		}
	}

	return nil, fmt.Errorf("模块未导出任何可识别的内存管理函数 (cabi_realloc, realloc, or malloc/free)")
}

// Allocate 在 Guest 中分配一块内存，会自动选择正确的分配方式。
func (a *GuestAllocator) Allocate(ctx context.Context, size, alignment uint32) (uint32, error) {
	switch a.mode {
	case modeCabiRealloc:
		// cabi_realloc(0, 0, alignment, size) 用于分配新内存
		results, err := a.realloc.Call(ctx, 0, 0, uint64(alignment), uint64(size))
		if err != nil {
			return 0, fmt.Errorf("cabi_realloc for allocate failed: %w", err)
		}
		return uint32(results[0]), nil

	case modeRealloc:
		// realloc(0, size) 用于分配新内存
		results, err := a.realloc.Call(ctx, 0, uint64(size))
		if err != nil {
			return 0, fmt.Errorf("realloc for allocate failed: %w", err)
		}
		return uint32(results[0]), nil

	case modeMallocFree:
		// malloc(size) 用于分配新内存
		results, err := a.malloc.Call(ctx, uint64(size))
		if err != nil {
			return 0, fmt.Errorf("malloc failed: %w", err)
		}
		return uint32(results[0]), nil

	default:
		return 0, fmt.Errorf("不支持的分配器模式")
	}
}

// Free 在 Guest 中释放一块内存，会自动选择正确的释放方式。
func (a *GuestAllocator) Free(ctx context.Context, ptr, size, alignment uint32) error {
	switch a.mode {
	case modeCabiRealloc:
		// cabi_realloc(ptr, size, alignment, 0) 用于释放内存
		_, err := a.realloc.Call(ctx, uint64(ptr), uint64(size), uint64(alignment), 0)
		if err != nil {
			return fmt.Errorf("cabi_realloc for free failed: %w", err)
		}
		return nil

	case modeRealloc:
		// 如果 free 函数存在 (C-style)，则调用 free(ptr)。
		// 如果 free 函数不存在 (TinyGo)，则这是一个安全无害的空操作。
		if a.free != nil {
			_, err := a.free.Call(ctx, uint64(ptr))
			if err != nil {
				return fmt.Errorf("free (paired with realloc) failed: %w", err)
			}
		}
		return nil

	case modeMallocFree:
		// free(ptr) 用于释放内存
		_, err := a.free.Call(ctx, uint64(ptr))
		if err != nil {
			return fmt.Errorf("free failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("不支持的分配器模式")
	}
}
