package v0_2

import (
	"context"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
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
	// body, ok := i.hm.IncomingBodies.Get(handle)
	// if !ok {
	// 	return
	// }
	// // 确保 Go 的 Response.Body 被关闭，以释放连接。
	// body.Body.Close()
	// i.hm.IncomingBodies.Remove(handle)
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
	return witgo.Ok[InputStream, witgo.Unit](body.StreamHandle)

	// readStream := manager_io.NewAsyncStreamForReader(body.Body)
	// body.StreamHandle = i.hm.Streams.Add(readStream)
}

// Finish 是一个静态方法，用于获取 trailers，我们暂时不实现。
func (i *incomingBodyImpl) Finish(this IncomingBody) FutureTrailers {
	// 1. 获取 incoming-body 实例。
	body, ok := i.hm.IncomingBodies.Get(this)
	if !ok {
		// 根据 WIT 规范，无效句柄应该触发陷阱。
		// 在 wazero 的 Go host function 中，panic 会被转换为 trap。
		panic("invalid incoming-body handle")
	}

	// 2. 检查 input-stream 是否仍然存在。
	if body.StreamHandle != 0 {
		if _, stillExists := i.hm.Streams.Get(body.StreamHandle); stillExists {
			// 3. 根据 WIT 规范，如果流仍然存在，则触发陷阱。
			panic("trap: finishing incoming-body while its stream is still alive")
		}
	}

	// 4. 获取 future-trailers 句柄，并消费（删除）incoming-body 资源。
	trailersHandle := body.Trailers
	i.hm.IncomingBodies.Remove(this)

	// 5. 返回 trailers 句柄。
	return trailersHandle
}
