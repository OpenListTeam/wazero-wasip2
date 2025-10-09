package v0_2

import (
	"context"
	"fmt"
	"io"
	"maps"

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
	if body, ok := i.hm.Bodies.Pop(handle); ok {
		// 关闭 BodyWriter 是关键。这会通知另一端的 PipeReader 数据已经结束。
		// 如果 finish() 没有被调用，这通常表示一个不完整的写入。
		body.BodyWriter.CloseWithError(io.EOF)
		i.hm.Streams.Remove(body.OutputStreamHandle)
	}
}

// output-stream必须在outgoing-body被丢弃或完成前先被丢弃，否则会触发错误（trap）；
// 该流只能获取一次（第一次调用成功），后续调用会返回错误（避免重复写入导致的混乱）。
func (i *outgoingBodyImpl) Write(_ context.Context, this OutgoingBody) witgo.Result[OutputStream, witgo.Unit] {
	body, ok := i.hm.Bodies.Get(this)
	if !ok {
		return witgo.Err[OutputStream, witgo.Unit](witgo.Unit{})
	}
	if !body.Consumed.CompareAndSwap(false, true) {
		return witgo.Err[OutputStream, witgo.Unit](witgo.Unit{})
	}
	return witgo.Ok[OutputStream, witgo.Unit](body.OutputStreamHandle)
}

// 显式标记消息体已完成，可附带 trailers。这是必须调用的方法，用于告知系统 "消息体内容已全部发送"
// 如果对应的 HTTP 请求 / 响应包含Content-Length头，finish会校验实际写入的内容长度是否与该头指定的值一致；不一致则返回失败（确保协议合规）
// 若未调用finish就直接丢弃outgoing-body资源，系统会将消息体视为 "不完整 / 损坏"，并通过各种方式（如破坏传输内容、中止请求、发送错误状态码）将错误反馈到 HTTP 协议层面
func (i *outgoingBodyImpl) Finish(_ context.Context, this OutgoingBody, trailers witgo.Option[Fields]) witgo.Result[witgo.Unit, ErrorCode] {
	body, ok := i.hm.Bodies.Pop(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCode{InternalError: witgo.SomePtr("invalid outgoing_body handle")})
	}

	// 处理 trailers
	if trailers.IsSome() {
		// 查找与此 body 关联的 OutgoingRequest
		if req, ok := i.hm.OutgoingRequests.Get(body.Request); ok {
			if tr, ok := i.hm.Fields.Pop(*trailers.Some); ok {
				maps.Copy(req.Trailers, tr)
			}
		}
	}

	// 关闭 BodyWriter，这将向 PipeReader 发出 EOF 信号，表示 body 已经写完。
	// 这也会确保所有在 AsyncWriteWrapper 缓冲区中的数据被刷出。
	body.BodyWriter.Close()
	i.hm.Streams.Remove(body.OutputStreamHandle)

	// 执行 Content-Length 校验
	if body.ContentLength != nil {
		bytesWritten := body.BytesWritten.Load()
		if bytesWritten != *body.ContentLength {
			errMsg := fmt.Sprintf("content-length mismatch: header specified %d, but %d bytes were written", *body.ContentLength, bytesWritten)
			return witgo.Err[witgo.Unit, ErrorCode](ErrorCode{HTTPProtocolError: &witgo.Unit{}, InternalError: witgo.SomePtr(errMsg)})
		}
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
