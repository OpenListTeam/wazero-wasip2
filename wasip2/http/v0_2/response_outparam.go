package v0_2

import (
	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

type responseOutparamImpl struct {
	hm *manager_http.HTTPManager
}

func newResponseOutparamImpl(hm *manager_http.HTTPManager) *responseOutparamImpl {
	return &responseOutparamImpl{hm: hm}
}

func (i *responseOutparamImpl) Drop(this ResponseOutparam) {
	i.hm.ResponseOutparams.Remove(this)
}

func (i *responseOutparamImpl) Set(param ResponseOutparam, response witgo.Result[OutgoingResponse, ErrorCode]) {
	p, ok := i.hm.ResponseOutparams.Pop(param)
	if !ok {
		return
	}

	if response.Err != nil {
		p.ResultChan <- *response.Err
	} else {
		p.ResultChan <- *response.Ok
	}
}
