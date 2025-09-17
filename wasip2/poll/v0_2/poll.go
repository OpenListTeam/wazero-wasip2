package v0_2

import (
	"context"
	"reflect"
	"wazero-wasip2/internal/poll"
)

// pollImpl 结构体持有 wasi:poll 的具体实现逻辑。
type pollImpl struct {
	pm *poll.Manager
}

// newPollImpl 创建一个新的 pollImpl 实例。
func newPollImpl(pm *poll.Manager) *pollImpl {
	return &pollImpl{pm: pm}
}

// DropPollable 是 pollable 资源的析构函数。
func (i *pollImpl) DropPollable(_ context.Context, handle Pollable) {
	i.pm.Remove(handle)
}

// Ready 实现 [method]pollable.ready 方法。它以非阻塞的方式检查 pollable 是否就绪。
func (i *pollImpl) Ready(_ context.Context, this Pollable) bool {
	ch, ok := i.pm.Get(this)
	if !ok {
		// 无效的句柄被认为是“就绪”的，以便调用者可以发现错误。
		return true
	}

	// 使用一个带 default 分支的 select 来实现非阻塞检查。
	select {
	case <-ch:
		// channel 已关闭，意味着 pollable 已经就绪。
		return true
	default:
		// channel 还开着，意味着 pollable 尚未就绪。
		return false
	}
}

// Block 实现 [method]pollable.block 方法。它会阻塞直到 pollable 就绪。
func (i *pollImpl) Block(_ context.Context, this Pollable) {
	ch, ok := i.pm.Get(this)
	if !ok {
		// 如果句柄无效，立即返回。
		return
	}
	// 阻塞式地等待 channel 关闭。
	<-ch
}

// Poll 实现了顶层的 poll 函数，它可以同时等待多个 pollable。
func (i *pollImpl) Poll(_ context.Context, handles []Pollable) []uint32 {
	// 根据 WIT 规范，如果列表为空，必须 trap。
	if len(handles) == 0 {
		panic("poll input list cannot be empty")
	}

	// 1. 检查是否已经有就绪的 pollable
	var readyIndexes []uint32
	cases := make([]reflect.SelectCase, len(handles))
	for j, handle := range handles {
		ch, ok := i.pm.Get(handle)
		if !ok {
			// 如果句柄无效，我们立即将其视为就绪。
			readyIndexes = append(readyIndexes, uint32(j))
			ch = make(chan struct{}) // 使用一个已关闭的 channel
			close(ch)
		} else {
			// 非阻塞检查
			select {
			case <-ch:
				readyIndexes = append(readyIndexes, uint32(j))
			default:
				// 尚未就绪
			}
		}
		cases[j] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}

	// 如果在第一次检查中就发现了就绪的 pollable，则立即返回。
	if len(readyIndexes) > 0 {
		return readyIndexes
	}

	// 2. 如果没有任何 pollable 就绪，则阻塞等待第一个就绪的事件。
	chosen, _, _ := reflect.Select(cases)

	return []uint32{uint32(chosen)}
}
