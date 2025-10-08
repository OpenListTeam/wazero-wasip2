package v0_2

import (
	gohttp "net/http"
	"strconv"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type outgoingResponseImpl struct {
	hm *manager_http.HTTPManager
}

func newOutgoingResponseImpl(hm *manager_http.HTTPManager) *outgoingResponseImpl {
	return &outgoingResponseImpl{hm: hm}
}

func (i *outgoingResponseImpl) Constructor(headers Headers) OutgoingResponse {
	resp := &manager_http.OutgoingResponse{
		StatusCode: gohttp.StatusOK,
		Headers:    headers,
	}

	id := i.hm.OutgoingResponses.Add(resp)

	var contentLength *uint64
	if headerFields, ok := i.hm.Fields.Get(headers); ok {
		if cl, ok := headerFields["content-length"]; ok && len(cl) > 0 {
			if val, err := strconv.ParseUint(cl[0], 10, 64); err == nil {
				contentLength = &val
			}
		}
	}

	bodyID, bodyReader, bodyWriter := i.hm.NewOutgoingBody(id, contentLength)
	resp.BodyHandle = bodyID
	resp.Body = bodyReader
	resp.BodyWriter = bodyWriter

	return id
}

func (i *outgoingResponseImpl) Drop(this OutgoingResponse) {
	if resp, ok := i.hm.OutgoingResponses.Get(this); ok {
		if resp.BodyHandle != 0 {
			i.hm.Bodies.Remove(resp.BodyHandle)
		}
	}
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
	return resp.Headers
}

func (i *outgoingResponseImpl) Body(this OutgoingResponse) witgo.Result[OutgoingBody, witgo.Unit] {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	return witgo.Ok[OutgoingBody, witgo.Unit](resp.BodyHandle)
}
