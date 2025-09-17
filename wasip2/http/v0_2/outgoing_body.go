package v0_2

import (
	"context"
	"wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type outgoingBodyImpl struct {
	hm *http.HTTPManager
}

func newOutgoingBodyImpl(hm *http.HTTPManager) *outgoingBodyImpl {
	return &outgoingBodyImpl{hm: hm}
}

// Drop 是 outgoing-body 资源的析构函数。
// 它确保在句柄被废弃时，相关的写入管道被关闭。
func (i *outgoingBodyImpl) Drop(_ context.Context, handle OutgoingBody) {
	body, ok := i.hm.Bodies.Get(handle)
	if !ok {
		return
	}
	// 关闭 BodyWriter 是关键。这会通知另一端的 PipeReader 数据已经结束。
	// 如果 finish() 没有被调用，这通常表示一个不完整的写入。
	body.BodyWriter.Close()
	i.hm.Bodies.Remove(handle)
}

func (i *outgoingBodyImpl) Write(_ context.Context, this OutgoingBody) witgo.Result[OutputStream, witgo.Unit] {
	body, ok := i.hm.Bodies.Get(this)
	if !ok {
		return witgo.Err[OutputStream, witgo.Unit](witgo.Unit{})
	}
	return witgo.Ok[OutputStream, witgo.Unit](body.OutputStreamHandle)
}

func (i *outgoingBodyImpl) Finish(_ context.Context, this OutgoingBody, trailers witgo.Option[Fields]) witgo.Result[witgo.Unit, ErrorCode] {
	body, ok := i.hm.Bodies.Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCode{InternalError: witgo.SomePtr("invalid outgoing_body handle")})
	}
	// 关闭 PipeWriter，这将向 PipeReader 发出 EOF 信号，表示 body 已经写完。
	body.BodyWriter.Close()
	// TODO: 处理 trailers
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
