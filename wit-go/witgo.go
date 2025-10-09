package witgo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"

	"github.com/tetratelabs/wazero/api"
)

// Host provides a high-level interface to interact with a Wasm component.
type Host struct {
	module    api.Module
	allocator *GuestAllocator
}

// NewHost creates a new Host instance for the given Wasm module.
func NewHost(module api.Module) (*Host, error) {
	alloc, err := NewGuestAllocator(module)
	if err != nil {
		return nil, err
	}
	return &Host{module: module, allocator: alloc}, nil
}

var ErrNotExportFunc = errors.New("guest function not exports")

// Call 调用一个导出的 Guest 函数。
// 它会自动检测 Guest 的 ABI 风格（扁平化参数 vs 单一结构体指针），
// 并相应地处理参数的提升（lifting）和结果的降低（lowering）。
func (h *Host) Call(ctx context.Context, funcName string, resultPtr interface{}, params ...interface{}) error {
	fn := h.module.ExportedFunction(funcName)
	if fn == nil {
		return fmt.Errorf("函数 '%s' 在 Guest 导出中未找到 %w", funcName, ErrNotExportFunc)
	}

	paramDefs := fn.Definition().ParamTypes()
	var flatParams []uint64

	// --- ABI 风格检测 ---
	// 用于检测“单一结构体指针”ABI 风格的启发式规则：
	// 1. Guest 函数只期望接收一个参数。
	// 2. 该参数是 32 位整数（即指针）。
	// 3. 用户在调用 Call() 时也只提供了一个参数。
	// 4. 该参数是结构体或指向结构体的指针，【并且不是一个 flags 类型】。
	isSingleStructPtrABI := false
	if len(paramDefs) == 1 && paramDefs[0] == api.ValueTypeI32 && len(params) == 1 && params[0] != nil {
		val := reflect.ValueOf(params[0])
		valType := val.Type()
		kind := valType.Kind()

		if kind == reflect.Ptr {
			if !val.IsNil() {
				valType = valType.Elem()
				kind = valType.Kind()
			}
		}

		if kind == reflect.Struct && !isFlags(valType) && !isVariant(valType) {
			isSingleStructPtrABI = true
		}
	}

	if isSingleStructPtrABI {
		// 新风格：提升（lift）单个结构体参数以获取一个指针。
		ptr, err := Lift(ctx, h, reflect.ValueOf(params[0]))
		if err != nil {
			return fmt.Errorf("为函数 '%s' 提升单个结构体参数失败: %w", funcName, err)
		}
		flatParams = []uint64{uint64(ptr)}
	} else {
		// 原始风格：扁平化所有参数。
		flatParams = make([]uint64, 0, 16)
		for _, p := range params {
			err := h.flattenParam(ctx, reflect.ValueOf(p), &flatParams)
			if err != nil {
				return fmt.Errorf("扁平化参数 %#v 失败: %w", p, err)
			}
		}
	}

	results, err := fn.Call(ctx, flatParams...)
	if err != nil {
		return fmt.Errorf("Guest 函数 '%s' 调用失败: %w", funcName, err)
	}

	if resultPtr != nil {
		if len(results) == 0 {
			if len(fn.Definition().ResultTypes()) > 0 {
				return fmt.Errorf("函数期望有返回值，但实际没有返回")
			}
			return nil
		}

		outVal := reflect.ValueOf(resultPtr).Elem()
		resultValue := results[0]

		switch outVal.Kind() {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			outVal.SetUint(resultValue)
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			outVal.SetInt(int64(resultValue))
		case reflect.Float32:
			outVal.SetFloat(float64(math.Float32frombits(uint32(resultValue))))
		case reflect.Float64:
			outVal.SetFloat(math.Float64frombits(resultValue))
		case reflect.Bool:
			outVal.SetBool(resultValue != 0)
		default:
			// 对于复杂类型（如结构体、字符串、切片等），返回值是一个指针。
			ptr := uint32(resultValue)
			err = Lower(ctx, h, ptr, outVal)
			if err != nil {
				return fmt.Errorf("failed to lower complex result from ptr %d: %w", ptr, err)
			}
		}
	}

	return nil
}
