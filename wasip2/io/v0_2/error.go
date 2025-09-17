package v0_2

import (
	"context"
	"wazero-wasip2/internal/io"
)

// errorImpl 结构体持有 wasi:io/error 的具体实现逻辑。
type errorImpl struct {
	em *io.ErrorManager
}

func newErrorImpl(em *io.ErrorManager) *errorImpl {
	return &errorImpl{em: em}
}

// DropError 是 error 资源的析构函数。
func (i *errorImpl) DropError(_ context.Context, handle Error) {
	i.em.Remove(handle)
}

// ToDebugString 实现了 [method]error.to-debug-string 方法。
func (i *errorImpl) ToDebugString(_ context.Context, this Error) string {
	if err, ok := i.em.Get(this); ok {
		return err.Error()
	}
	return "invalid error handle"
}
