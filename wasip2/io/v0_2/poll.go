package v0_2

import (
	"context"
	"reflect"
	"wazero-wasip2/internal/io"
)

// pollImpl 结构体持有 wasi:io/poll 的具体实现逻辑。
type pollImpl struct {
	pm *io.PollManager
}

func newPollImpl(pm *io.PollManager) *pollImpl {
	return &pollImpl{pm: pm}
}

// DropPollable 是 pollable 资源的析构函数。
func (i *pollImpl) DropPollable(_ context.Context, handle Pollable) {
	if p, ok := i.pm.Get(handle); ok && p.Cancel != nil {
		p.Cancel()
	}
	i.pm.Remove(handle)
}

// Ready 实现 [method]pollable.ready 方法。
func (i *pollImpl) Ready(_ context.Context, this Pollable) bool {
	p, ok := i.pm.Get(this)
	if !ok {
		return true
	}
	return p.IsReady()
}

// Block 实现 [method]pollable.block 方法。
func (i *pollImpl) Block(_ context.Context, this Pollable) {
	p, ok := i.pm.Get(this)
	if !ok {
		return
	}
	p.Block()
}

func (i *pollImpl) Poll(_ context.Context, handles []Pollable) []uint32 {
	if len(handles) == 0 {
		panic("poll input list cannot be empty")
	}

	// 1. 快速非阻塞检查
	var readyIndexes []uint32
	for j, handle := range handles {
		if i.Ready(context.Background(), handle) {
			readyIndexes = append(readyIndexes, uint32(j))
		}
	}
	if len(readyIndexes) > 0 {
		return readyIndexes
	}

	// 2. 构造 select cases 并阻塞
	cases := make([]reflect.SelectCase, len(handles))
	for j, handle := range handles {
		ch := make(chan struct{}) // 默认创建一个永不就绪的 channel
		if p, ok := i.pm.Get(handle); ok {
			ch = p.Channel()
		}
		cases[j] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}

	chosen, _, _ := reflect.Select(cases)
	return []uint32{uint32(chosen)}
}
