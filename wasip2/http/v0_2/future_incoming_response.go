package v0_2

import (
	"context"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	manager_io "github.com/foxxorcat/wazero-wasip2/manager/io"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
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

	future.PollableOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		// 创建一个新的 pollable，而不是裸 channel
		future.Pollable = manager_io.NewPollable(cancel)
		go func() {
			select {
			case <-ctx.Done():
				return
			case res, ok := <-future.ResultChan:
				if ok {
					future.Result.Store(&res) // 原子性地存储结果
				}
				future.Pollable.SetReady() // 在就绪时调用 SetReady
			}
		}()
	})

	return i.hm.Poll.Add(future.Pollable)
}

// Get implements [method]future-incoming-response.get.
// It returns the response at most once.
func (i *futureIncomingResponseImpl) Get(
	_ context.Context,
	this FutureIncomingResponse,
) witgo.Option[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]] {
	future, ok := i.hm.Futures.Get(this)
	if !ok {
		// Invalid handle, return None. The WIT doesn't specify an error here.
		return witgo.None[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]]()
	}

	res := future.Result.Load() // Atomically load the result
	if res == nil {
		// Future is not ready yet.
		return witgo.None[witgo.Result[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit]]()
	}

	// Try to consume the result.
	if !future.Consumed.CompareAndSwap(false, true) {
		// It was already consumed. Return Some(Err()).
		outerResult := witgo.Err[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit](witgo.Unit{})
		return witgo.Some(outerResult)
	}

	// First time consuming.
	var innerResult witgo.Result[IncomingResponse, ErrorCode]
	if res.Err != nil {
		// The HTTP request itself failed.
		innerResult = witgo.Err[IncomingResponse, ErrorCode](ErrorCode{InternalError: witgo.SomePtr(res.Err.Error())})
	} else {
		// The HTTP request succeeded.
		innerResult = witgo.Ok[IncomingResponse, ErrorCode](res.ResponseHandle)
	}

	// Wrap the inner result in Ok() to signify a successful 'get' operation.
	outerResult := witgo.Ok[witgo.Result[IncomingResponse, ErrorCode], witgo.Unit](innerResult)
	return witgo.Some(outerResult)
}
