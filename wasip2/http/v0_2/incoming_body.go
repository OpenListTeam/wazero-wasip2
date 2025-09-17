package v0_2

import (
	"context"
	"wazero-wasip2/internal/http"
	"wazero-wasip2/internal/streams"
	witgo "wazero-wasip2/wit-go"
)

type incomingBodyImpl struct {
	hm *http.HTTPManager
}

func newIncomingBodyImpl(hm *http.HTTPManager) *incomingBodyImpl {
	return &incomingBodyImpl{hm: hm}
}

// Drop 是析构函数
func (i *incomingBodyImpl) Drop(_ context.Context, handle IncomingBody) {
	body, ok := i.hm.IncomingBodies.Get(handle)
	if !ok {
		return
	}
	// 确保 Go 的 Response.Body 被关闭，以释放连接。
	body.Body.Close()
	i.hm.IncomingBodies.Remove(handle)
}

// Stream 实现了 [method]incoming-body.stream。
// 将响应体转换为 wasi:io 的 input-stream。
func (i *incomingBodyImpl) Stream(_ context.Context, this IncomingBody) witgo.Result[InputStream, witgo.Unit] {
	body, ok := i.hm.IncomingBodies.Get(this)
	if !ok {
		return witgo.Err[InputStream, witgo.Unit](witgo.Unit{})
	}
	if body.StreamHandle != 0 {
		return witgo.Ok[InputStream, witgo.Unit](body.StreamHandle)
	}

	// 将 http.Response.Body (io.ReadCloser) 封装成我们的 Stream 对象。
	s := &streams.Stream{Reader: body.Body, Closer: body.Body}
	handle := i.hm.Streams.Add(s)
	body.StreamHandle = handle
	return witgo.Ok[InputStream, witgo.Unit](handle)
}

// Finish 是一个静态方法，用于获取 trailers，我们暂时不实现。
func (i *incomingBodyImpl) Finish(this IncomingBody) FutureTrailers {
	return 0
}
