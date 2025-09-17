package v0_2

import (
	"context"
	"io"
	"wazero-wasip2/internal/http"
	"wazero-wasip2/internal/streams"
	witgo "wazero-wasip2/wit-go"
)

type outgoingRequestImpl struct {
	hm *http.HTTPManager
}

func newOutgoingRequestImpl(hm *http.HTTPManager) *outgoingRequestImpl {
	return &outgoingRequestImpl{hm: hm}
}

func (i *outgoingRequestImpl) Constructor(fields Fields) OutgoingRequest {
	pr, pw := io.Pipe()
	req := &http.OutgoingRequest{
		Headers:    fields,
		Body:       pr,
		BodyWriter: pw,
	}
	return i.hm.OutgoingRequests.Add(req)
}

func (i *outgoingRequestImpl) Drop(_ context.Context, handle OutgoingRequest) {
	i.hm.OutgoingRequests.Remove(handle)
}

func (i *outgoingRequestImpl) Body(_ context.Context, this OutgoingRequest) witgo.Result[OutgoingBody, witgo.Unit] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	if req.BodyHandle != 0 {
		return witgo.Ok[OutgoingBody, witgo.Unit](req.BodyHandle)
	}

	stream := &streams.Stream{Writer: req.BodyWriter, Closer: req.BodyWriter}
	streamHandle := i.hm.Streams.Add(stream)

	body := &http.OutgoingBody{
		OutputStreamHandle: streamHandle,
		BodyWriter:         req.BodyWriter,
	}
	bodyHandle := i.hm.Bodies.Add(body)
	req.BodyHandle = bodyHandle
	return witgo.Ok[OutgoingBody, witgo.Unit](bodyHandle)
}

// --- Getters ---
func (i *outgoingRequestImpl) Method(_ context.Context, this OutgoingRequest) Method {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return Method{Other: witgo.String("")}
	}
	return toWasiMethod(req.Method)
}

func (i *outgoingRequestImpl) PathWithQuery(_ context.Context, this OutgoingRequest) witgo.Option[string] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok || req.Path == "" {
		return witgo.None[string]()
	}
	return witgo.Some(req.Path)
}

func (i *outgoingRequestImpl) Scheme(_ context.Context, this OutgoingRequest) witgo.Option[Scheme] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok || req.Scheme == nil {
		return witgo.None[Scheme]()
	}
	return witgo.Some(toWasiScheme(*req.Scheme))
}

func (i *outgoingRequestImpl) Authority(_ context.Context, this OutgoingRequest) witgo.Option[string] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok || req.Authority == nil {
		return witgo.None[string]()
	}
	return witgo.Some(*req.Authority)
}

func (i *outgoingRequestImpl) Headers(_ context.Context, this OutgoingRequest) Headers {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return 0
	}
	return req.Headers
}

// --- Setters ---
func (i *outgoingRequestImpl) SetMethod(_ context.Context, this OutgoingRequest, method Method) witgo.UnitResult {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	req.Method = fromWasiMethod(method)
	return witgo.UintOk()
}

func (i *outgoingRequestImpl) SetPathWithQuery(_ context.Context, this OutgoingRequest, path witgo.Option[string]) witgo.UnitResult {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	if path.Some != nil {
		req.Path = *path.Some
	} else {
		req.Path = ""
	}
	return witgo.UintOk()
}

func (i *outgoingRequestImpl) SetScheme(_ context.Context, this OutgoingRequest, scheme witgo.Option[Scheme]) witgo.UnitResult {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	if scheme.Some != nil {
		req.Scheme = fromWasiScheme(*scheme.Some)
	} else {
		req.Scheme = nil
	}
	return witgo.UintOk()
}

func (i *outgoingRequestImpl) SetAuthority(_ context.Context, this OutgoingRequest, authority witgo.Option[string]) witgo.UnitResult {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	req.Authority = authority.Some
	return witgo.UintOk()
}
