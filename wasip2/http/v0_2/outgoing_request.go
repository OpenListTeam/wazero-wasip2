package v0_2

import (
	"context"
	"maps"
	"strconv"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

// guest 发起请求 host 处理
type outgoingRequestImpl struct {
	hm *manager_http.HTTPManager
}

func newOutgoingRequestImpl(hm *manager_http.HTTPManager) *outgoingRequestImpl {
	return &outgoingRequestImpl{hm: hm}
}

func (i *outgoingRequestImpl) Constructor(fields Fields) OutgoingRequest {
	// 消耗掉资源
	header, _ := i.hm.Fields.Pop(fields)
	req := &manager_http.OutgoingRequest{
		Headers: header,
	}
	return i.hm.OutgoingRequests.Add(req)
}

func (i *outgoingRequestImpl) Drop(_ context.Context, handle OutgoingRequest) {
	i.hm.OutgoingRequests.Remove(handle)
}

// 返回当前请求对应的输出体（outgoing-body）资源。
// 仅首次调用成功，最多获取一次；后续调用返回错误。
func (i *outgoingRequestImpl) Body(_ context.Context, this OutgoingRequest) witgo.Result[OutgoingBody, witgo.Unit] {
	req, ok := i.hm.OutgoingRequests.Get(this)
	if !ok {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}
	if !req.Consumed.CompareAndSwap(false, true) {
		return witgo.Err[OutgoingBody, witgo.Unit](witgo.Unit{})
	}

	var contentLength *uint64
	if cl := req.Headers.Get("Content-Length"); len(cl) > 0 {
		if val, err := strconv.ParseUint(cl, 10, 64); err == nil {
			contentLength = &val
		}
	}
	req.BodyHandle, req.Body, req.BodyWriter = i.hm.NewOutgoingBody(contentLength, func(trailers manager_http.Fields) error {
		if req.Request != nil {
			if req.Request.Trailer != nil {
				maps.Copy(req.Request.Trailer, trailers)
			} else {
				req.Request.Trailer = trailers
			}
		}
		return nil
	})
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
	return i.hm.Fields.Add(manager_http.Fields(req.Headers))
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
