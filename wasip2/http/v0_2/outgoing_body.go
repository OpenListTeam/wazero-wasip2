package v0_2

import (
	"context"
	"fmt"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type outgoingBodyImpl struct {
	hm *manager_http.HTTPManager
}

func newOutgoingBodyImpl(hm *manager_http.HTTPManager) *outgoingBodyImpl {
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

	// 关闭 BodyWriter，这将向 PipeReader 发出 EOF 信号，表示 body 已经写完。
	// 这也会确保所有在 AsyncWriteWrapper 缓冲区中的数据被刷出。
	body.BodyWriter.Close()

	// 【新增】执行 Content-Length 校验
	if body.ContentLength != nil {
		bytesWritten := body.BytesWritten.Load()
		if bytesWritten != *body.ContentLength {
			errMsg := fmt.Sprintf("content-length mismatch: header specified %d, but %d bytes were written", *body.ContentLength, bytesWritten)
			return witgo.Err[witgo.Unit, ErrorCode](ErrorCode{HTTPProtocolError: &witgo.Unit{}, InternalError: witgo.SomePtr(errMsg)})
		}
	}

	// TODO: trailers 的处理
	if trailers.Some != nil {
		// 具体的实现逻辑需要根据是 request 还是 response 来定。
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
