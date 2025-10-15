package v0_2

import (
	gohttp "net/http"
	"strconv"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type outgoingResponseImpl struct {
	hm *manager_http.HTTPManager
}

func newOutgoingResponseImpl(hm *manager_http.HTTPManager) *outgoingResponseImpl {
	return &outgoingResponseImpl{hm: hm}
}

func (i *outgoingResponseImpl) Constructor(headers Headers) OutgoingResponse {
	header, _ := i.hm.Fields.Pop(headers)
	resp := &manager_http.OutgoingResponse{
		StatusCode: gohttp.StatusOK,
		Headers:    header,
	}

	return i.hm.OutgoingResponses.Add(resp)
}

func (i *outgoingResponseImpl) Drop(this OutgoingResponse) {
	i.hm.OutgoingResponses.Remove(this)
}

func (i *outgoingResponseImpl) StatusCode(this OutgoingResponse) StatusCode {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return 0
	}
	return StatusCode(resp.StatusCode)
}

func (i *outgoingResponseImpl) SetStatusCode(this OutgoingResponse, statusCode StatusCode) witgo.Result[witgo.Unit, witgo.Unit] {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, witgo.Unit](witgo.Unit{})
	}
	resp.StatusCode = int(statusCode)
	return witgo.Ok[witgo.Unit, witgo.Unit](witgo.Unit{})
}

func (i *outgoingResponseImpl) Headers(this OutgoingResponse) Headers {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return 0
	}
	return i.hm.Fields.Add(manager_http.Fields(resp.Headers))
}

func (i *outgoingResponseImpl) Body(this OutgoingResponse) witgo.Result[OutgoingBody, witgo.Unit] {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	if !resp.Consumed.CompareAndSwap(false, true) {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}

	var contentLength *uint64
	if cl := resp.Headers.Get("Content-Length"); len(cl) > 0 {
		if val, err := strconv.ParseUint(cl, 10, 64); err == nil {
			contentLength = &val
		}
	}

	resp.BodyHandle, resp.Body, resp.BodyWriter = i.hm.NewOutgoingBody(contentLength, func(trailers manager_http.Fields) error {
		if resp.Response != nil {
			return trailers.Write(resp.Response)
		}
		return nil
	})
	return witgo.Ok[OutgoingBody, witgo.Unit](resp.BodyHandle)
}
