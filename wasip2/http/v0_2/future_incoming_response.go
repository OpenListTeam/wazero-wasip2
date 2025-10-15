package v0_2

import (
	"context"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type futureIncomingResponseImpl struct {
	hm *manager_http.HTTPManager
}

func newFutureIncomingResponseImpl(hm *manager_http.HTTPManager) *futureIncomingResponseImpl {
	return &futureIncomingResponseImpl{hm: hm}
}

// Drop 是 future-incoming-response 资源的析构函数。
func (i *futureIncomingResponseImpl) Drop(_ context.Context, handle FutureIncomingResponse) {
	i.hm.Futures.Remove(handle)
}

// Subscribe 实现了 [method]future-incoming-response.subscribe。
func (i *futureIncomingResponseImpl) Subscribe(_ context.Context, this FutureIncomingResponse) Pollable {
	future, ok := i.hm.Futures.Get(this)
	if !ok {
		// 对于无效句柄，返回一个立即就绪的 pollable
		return i.hm.Poll.Add(manager_io.ReadyPollable)
	}

	return i.hm.Poll.Add(future.Pollable)
}

// Get implements [method]future-incoming-response.get.
// It returns the response at most once.
func (i *futureIncomingResponseImpl) Get(
	ctx context.Context,
	this FutureIncomingResponse,
) witgo.Option[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]] {
	future, ok := i.hm.Futures.Get(this)
	if !ok {
		// Invalid handle, return None. The WIT doesn't specify an error here.
		return witgo.None[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]]()
	}

	select {
	case <-future.Pollable.Channel():
	case <-ctx.Done():
		return witgo.None[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]]()
	}

	if !future.Consumed.CompareAndSwap(false, true) {
		// It was already consumed. Return Some(Err()).
		outerResult := witgo.Err[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit](witgo.Unit{})
		return witgo.Some(outerResult)
	}

	var innerResult witgo.Result[IncomingResponse, ErrorCode]
	if future.Result.Err != nil {
		innerResult = witgo.Err[IncomingResponse, ErrorCode](mapGoErrToWasiHttpErr(future.Result.Err))
	} else {
		responseHandle := i.hm.Responses.Add(&manager_http.IncomingResponse{
			Response: future.Result.Response,

			StatusCode: future.Result.Response.StatusCode,
			Headers:    future.Result.Response.Header,
		})
		innerResult = witgo.Ok[IncomingResponse, ErrorCode](responseHandle)
	}

	// Wrap the inner result in Ok() to signify a successful 'get' operation.
	outerResult := witgo.Ok[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit](innerResult)
	return witgo.Some(outerResult)
}
