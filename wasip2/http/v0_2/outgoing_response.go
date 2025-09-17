package v0_2

import (
	"io"
	gohttp "net/http"
	"wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type outgoingResponseImpl struct {
	hm *http.HTTPManager
}

func newOutgoingResponseImpl(hm *http.HTTPManager) *outgoingResponseImpl {
	return &outgoingResponseImpl{hm: hm}
}

func (i *outgoingResponseImpl) Constructor(headers Headers) OutgoingResponse {
	// Create the in-memory pipe. The guest writes to 'w', the host reads from 'r'.
	r, w := io.Pipe()

	resp := &http.OutgoingResponse{
		StatusCode: gohttp.StatusOK,
		Headers:    headers,
		Body:       r, // Store the reader for the host to use.
		BodyWriter: w, // Store the writer for the guest's stream.
	}

	// Create the resource for the response itself.
	id := i.hm.OutgoingResponses.Add(resp)

	// Create the resource for the outgoing-body.
	bodyID := i.hm.Bodies.Add(&http.OutgoingBody{
		BodyWriter: w,
		Request:    id, // Link body back to its parent response.
	})

	// Associate the body handle with the response.
	resp.BodyHandle = bodyID

	return id
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
	return resp.Headers
}

func (i *outgoingResponseImpl) Body(this OutgoingResponse) witgo.Result[OutgoingBody, witgo.Unit] {
	resp, ok := i.hm.OutgoingResponses.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	return witgo.Ok[OutgoingBody, witgo.Unit](resp.BodyHandle)
}
