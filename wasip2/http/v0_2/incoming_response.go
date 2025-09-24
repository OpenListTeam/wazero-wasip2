package v0_2

import (
	"context"
	manager_http "wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type incomingResponseImpl struct {
	hm *manager_http.HTTPManager
}

func newIncomingResponseImpl(hm *manager_http.HTTPManager) *incomingResponseImpl {
	return &incomingResponseImpl{hm: hm}
}

// Drop 是析构函数
func (i *incomingResponseImpl) Drop(_ context.Context, handle IncomingResponse) {
	i.hm.Responses.Remove(handle)
}

// Status 实现了 [method]incoming-response.status。
func (i *incomingResponseImpl) Status(_ context.Context, this IncomingResponse) uint16 {
	resp, ok := i.hm.Responses.Get(this)
	if !ok {
		return 0
	}
	return uint16(resp.Response.StatusCode)
}

// Headers 实现了 [method]incoming-response.headers。
func (i *incomingResponseImpl) Headers(_ context.Context, this IncomingResponse) Fields {
	resp, ok := i.hm.Responses.Get(this)
	if !ok {
		return 0
	}
	return resp.Headers
}

// Consume 实现了 [method]incoming-response.consume。
// 返回一个 incoming-body，用于读取响应体。
func (i *incomingResponseImpl) Consume(_ context.Context, this IncomingResponse) witgo.Result[IncomingBody, witgo.Unit] {
	resp, ok := i.hm.Responses.Get(this)
	if !ok {
		return witgo.Err[IncomingBody, witgo.Unit](witgo.Unit{})
	}
	if resp.BodyConsumed {
		// body 已经被消费过一次，不能重复消费。
		return witgo.Err[IncomingBody, witgo.Unit](witgo.Unit{})
	}
	resp.BodyConsumed = true

	// 创建一个 incoming-body 资源来包装 Go 的 manager_http.Response.Body
	// body := &manager_http.IncomingBody{Body: resp.Response.Body}
	// handle := i.hm.IncomingBodies.Add(body)
	// resp.BodyHandle = handle

	return witgo.Ok[IncomingBody, witgo.Unit](resp.BodyHandle)
}
