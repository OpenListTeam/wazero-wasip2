package v0_2

import (
	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type incomingRequestImpl struct {
	hm *manager_http.HTTPManager
}

func newIncomingRequestImpl(hm *manager_http.HTTPManager) *incomingRequestImpl {
	return &incomingRequestImpl{hm: hm}
}

func (i *incomingRequestImpl) Drop(this IncomingRequest) {
	i.hm.IncomingRequests.Remove(this)
}

func (i *incomingRequestImpl) Method(this IncomingRequest) Method {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return Method{Other: new(string)} // Should not happen
	}
	return toWasiMethod(req.Method)
}

func (i *incomingRequestImpl) PathWithQuery(this IncomingRequest) witgo.Option[string] {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return witgo.None[string]()
	}
	path := req.Path
	if req.Query != "" {
		path += "?" + req.Query
	}
	return witgo.Some(path)
}

func (i *incomingRequestImpl) Scheme(this IncomingRequest) witgo.Option[Scheme] {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return witgo.None[Scheme]()
	}
	if req.Scheme == nil {
		return witgo.None[Scheme]()
	}
	return witgo.Some(toWasiScheme(*req.Scheme))
}

func (i *incomingRequestImpl) Authority(this IncomingRequest) witgo.Option[string] {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return witgo.None[string]()
	}
	if req.Authority == nil {
		return witgo.None[string]()
	}
	return witgo.Some(*req.Authority)
}

func (i *incomingRequestImpl) Headers(this IncomingRequest) Headers {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return 0
	}
	return req.Headers
}

func (i *incomingRequestImpl) Consume(this IncomingRequest) witgo.Result[IncomingBody, witgo.Unit] {
	req, ok := i.hm.IncomingRequests.Get(this)
	if !ok {
		return witgo.Err[IncomingBody, witgo.Unit](witgo.Unit{})
	}

	// TODO
	return witgo.Ok[IncomingBody, witgo.Unit](req.BodyHandle)
}
