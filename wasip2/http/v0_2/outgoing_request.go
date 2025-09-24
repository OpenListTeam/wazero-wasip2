package v0_2

import (
	"context"
	"strconv"
	manager_http "wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type outgoingRequestImpl struct {
	hm *manager_http.HTTPManager
}

func newOutgoingRequestImpl(hm *manager_http.HTTPManager) *outgoingRequestImpl {
	return &outgoingRequestImpl{hm: hm}
}

func (i *outgoingRequestImpl) Constructor(fields Fields) OutgoingRequest {
	req := &manager_http.OutgoingRequest{
		Headers: fields,
	}

	handle := i.hm.OutgoingRequests.Add(req)

	var contentLength *uint64
	if headerFields, ok := i.hm.Fields.Get(fields); ok {
		if cl, ok := headerFields["content-length"]; ok && len(cl) > 0 {
			if val, err := strconv.ParseUint(cl[0], 10, 64); err == nil {
				contentLength = &val
			}
		}
	}

	bodyHandle, bodyReader, bodyWriter := i.hm.NewOutgoingBody(handle, contentLength)
	req.BodyHandle = bodyHandle
	req.Body = bodyReader
	req.BodyWriter = bodyWriter

	return handle
}

func (i *outgoingRequestImpl) Drop(_ context.Context, handle OutgoingRequest) {
	if req, ok := i.hm.OutgoingRequests.Get(handle); ok {
		if req.BodyHandle != 0 {
			i.hm.Bodies.Remove(req.BodyHandle)
		}
	}
	i.hm.OutgoingRequests.Remove(handle)
}

func (i *outgoingRequestImpl) Body(_ context.Context, this OutgoingRequest) witgo.Result[OutgoingBody, witgo.Unit] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	if req.BodyHandle == 0 {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	return witgo.Ok[OutgoingBody, witgo.Unit](req.BodyHandle)
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
