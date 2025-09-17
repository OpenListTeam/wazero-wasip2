package v0_2

import (
	"wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type responseOutparamImpl struct {
	hm *http.HTTPManager
}

func newResponseOutparamImpl(hm *http.HTTPManager) *responseOutparamImpl {
	return &responseOutparamImpl{hm: hm}
}

func (i *responseOutparamImpl) Drop(this ResponseOutparam) {
	i.hm.ResponseOutparams.Remove(this)
}

func (i *responseOutparamImpl) Set(param ResponseOutparam, response witgo.Result[OutgoingResponse, ErrorCode]) {
	p, ok := i.hm.ResponseOutparams.Get(param)
	if !ok {
		return
	}
	defer i.hm.ResponseOutparams.Remove(param)

	if response.Err != nil {
		p.ResultChan <- *response.Err
	} else {
		p.ResultChan <- *response.Ok
	}
}
