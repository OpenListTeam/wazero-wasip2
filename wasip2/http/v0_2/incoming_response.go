package v0_2

import (
	"context"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
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
		panic("invalid incoming-respone handle")
	}
	return uint16(resp.Response.StatusCode)
}

// Headers 实现了 [method]incoming-response.headers。
func (i *incomingResponseImpl) Headers(_ context.Context, this IncomingResponse) Fields {
	resp, ok := i.hm.Responses.Get(this)
	if !ok {
		panic("invalid incoming-respone handle")
	}
	return i.hm.Fields.Add(manager_http.Fields(resp.Response.Header))
}

// Consume 实现了 [method]incoming-response.consume。
// 返回一个 incoming-body，用于读取响应体。
func (i *incomingResponseImpl) Consume(_ context.Context, this IncomingResponse) witgo.Result[IncomingBody, witgo.Unit] {
	resp, ok := i.hm.Responses.Get(this)
	if !ok {
		return witgo.Err[IncomingBody, witgo.Unit](witgo.Unit{})
	}
	if !resp.Consumed.CompareAndSwap(false, true) {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}

	body := &manager_http.IncomingBody{
		Stream: resp.Response.Body,
		GetTrailers: func() (trailers manager_http.Fields) {
			return resp.Response.Trailer
		},
	}
	resp.BodyHandle = i.hm.IncomingBodies.Add(body)
	return witgo.Ok[IncomingBody, witgo.Unit](resp.BodyHandle)
}
