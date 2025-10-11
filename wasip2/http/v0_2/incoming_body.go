package v0_2

import (
	"context"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	manager_io "github.com/foxxorcat/wazero-wasip2/manager/io"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type incomingBodyImpl struct {
	hm *manager_http.HTTPManager
}

func newIncomingBodyImpl(hm *manager_http.HTTPManager) *incomingBodyImpl {
	return &incomingBodyImpl{hm: hm}
}

// Drop 是析构函数
func (i *incomingBodyImpl) Drop(_ context.Context, handle IncomingBody) {
	i.hm.IncomingBodies.Remove(handle)
}

// Stream 实现了 [method]incoming-body.stream。
// 将响应体转换为 wasi:io 的 input-stream。
func (i *incomingBodyImpl) Stream(_ context.Context, this IncomingBody) witgo.Result[InputStream, witgo.Unit] {
	body, ok := i.hm.IncomingBodies.Get(this)
	if !ok {
		return witgo.Err[InputStream, witgo.Unit](witgo.Unit{})
	}

	if !body.Consumed.CompareAndSwap(false, true) {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}

	stream := manager_io.NewAsyncStreamForReader(body.Stream)
	body.StreamHandle = i.hm.Streams.Add(stream)
	return witgo.Ok[InputStream, witgo.Unit](body.StreamHandle)
}

// Finish 是一个静态方法
func (i *incomingBodyImpl) Finish(this IncomingBody) FutureTrailers {
	// 1. 获取 incoming-body 实例。
	body, ok := i.hm.IncomingBodies.Pop(this)
	if !ok {
		// 根据 WIT 规范，无效句柄应该触发陷阱。
		// 在 wazero 的 Go host function 中，panic 会被转换为 trap。
		panic("invalid incoming-body handle")
	}

	// 为了兼容，这里直接清理资源不报错
	if body.Consumed.CompareAndSwap(false, true) {
		// 3. 根据 WIT 规范，如果流仍然存在，则触发panic。
		// panic("trap: finishing incoming-body while its stream is still alive")
	}

	i.hm.Streams.Remove(body.StreamHandle)

	future := &manager_http.FutureTrailers{
		Pollable: manager_io.ReadyPollable,
	}
	if body.GetTrailers != nil {
		future.Result.Trailers = body.GetTrailers()
	}
	return i.hm.FutureTrailers.Add(future)
}
