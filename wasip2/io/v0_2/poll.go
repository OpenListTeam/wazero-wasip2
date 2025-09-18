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
		return []uint32{}
	}

	var readyIndexes []uint32
	var pollFds []pollFd

	for j, handle := range handles {
		if p, ok := i.pm.Get(handle); ok && p.IsReady() {
			readyIndexes = append(readyIndexes, uint32(j))
		} else if s, ok := i.pm.Get(handle); ok && s.Fd != 0 {
			// 如果是基于 Fd 的流，则添加到轮询列表
			pfd := pollFd{Fd: int32(s.Fd)}
			if s.Direction == io.PollDirectionRead {
				pfd.Events = pollEventRead
			} else {
				pfd.Events = pollEventWrite
			}
			pollFds = append(pollFds, pfd)
		}
	}

	if len(readyIndexes) > 0 {
		return readyIndexes
	}

	if len(pollFds) == 0 {
		// 没有可轮询的 Fd，阻塞直到其中一个 pollable 就绪
		// (例如，定时器)
		cases := make([]reflect.SelectCase, len(handles))
		for j, handle := range handles {
			ch := make(chan struct{})
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

	// 执行平台特定的轮询
	n, err := poll(pollFds, 100) // 100ms timeout
	if n == 0 || err != nil {
		return []uint32{}
	}

	for j, pfd := range pollFds {
		if (pfd.Revents&pfd.Events) != 0 || (pfd.Revents&pollEventError) != 0 {
			readyIndexes = append(readyIndexes, uint32(j))
		}
	}

	return readyIndexes
}
