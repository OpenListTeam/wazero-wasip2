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

	// 检查流是否已经被取走
	if body.StreamTaken {
		return witgo.Err[InputStream, witgo.Unit](witgo.Unit{})
	}
	body.StreamTaken = true // 标记为已取出

	// 将 http.Response.Body (io.ReadCloser) 封装成我们的 Stream 对象。
	s := &streams.Stream{Reader: body.Body, Closer: body.Body}
	handle := i.hm.Streams.Add(s)
	body.StreamHandle = handle
	return witgo.Ok[InputStream, witgo.Unit](handle)
}

// Finish 是一个静态方法，用于获取 trailers，我们暂时不实现。
func (i *incomingBodyImpl) Finish(this IncomingBody) FutureTrailers {
	// 1. 获取 incoming-body 实例。
	body, ok := i.hm.IncomingBodies.Get(this)
	if !ok {
		// 如果句柄无效，这本身就是一个错误，应该触发陷阱。
		panic("invalid incoming-body handle")
	}

	// 2. 检查 input-stream 是否仍然存在。
	// body.StreamHandle 在 Stream() 方法被调用后会被设为0，
	// 但我们需要检查 streams 管理器中是否还存在该资源实例。
	// 这需要我们知道原始的流句柄，因此我们需要在 IncomingBody 结构中
	// 保留原始句柄的副本，或者修改 Stream() 的逻辑。
	// 假设我们在 internal/http/http.go 的 IncomingBody 中添加了一个 UnconsumedStreamHandle 字段
	// 来跟踪原始句柄。
	//
	// 一个更简单的实现是检查 streams 管理器中是否存在这个句柄。
	// 如果 Stream() 方法没有被调用，body.StreamHandle 不会是0。
	if body.StreamHandle != 0 {
		if _, stillExists := i.hm.Streams.Get(body.StreamHandle); stillExists {
			// 3. 如果流仍然存在，触发陷阱。
			panic("trap: finishing incoming-body while its stream is still alive")
		}
	}

	// 4. 获取 future-trailers 句柄，并消费（删除）incoming-body 资源。
	trailersHandle := body.Trailers
	i.hm.IncomingBodies.Remove(this)

	// 5. 返回 trailers 句柄。
	return trailersHandle
}
