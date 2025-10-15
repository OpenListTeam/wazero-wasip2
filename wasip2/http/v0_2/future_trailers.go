package v0_2

import (
	"context"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type futureTrailersImpl struct {
	hm *manager_http.HTTPManager
}

func newFutureTrailersImpl(hm *manager_http.HTTPManager) *futureTrailersImpl {
	return &futureTrailersImpl{hm: hm}
}

func (i *futureTrailersImpl) Drop(this FutureTrailers) {
	i.hm.FutureTrailers.Remove(this)
}

func (i *futureTrailersImpl) Subscribe(this FutureTrailers) Pollable {
	future, ok := i.hm.FutureTrailers.Get(this)
	if !ok {
		// 对于无效句柄，返回一个立即就绪的 pollable
		return i.hm.Poll.Add(manager_io.ReadyPollable)
	}

	return i.hm.Poll.Add(future.Pollable)
}

func (i *futureTrailersImpl) Get(ctx context.Context, this FutureTrailers) witgo.Option[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]] {
	future, ok := i.hm.FutureTrailers.Get(this)
	if !ok {
		return witgo.None[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]]()
	}

	select {
	case <-future.Pollable.Channel():
	case <-ctx.Done():
		return witgo.None[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]]()
	}

	if !future.Consumed.CompareAndSwap(false, true) {
		// It was already consumed. Return Some(Err()).
		return witgo.None[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]]()
	}

	if future.Result.Err != nil {
		// 读取 body 过程中发生错误
		errorCode := ErrorCode{InternalError: witgo.SomePtr(future.Result.Err.Error())}
		return witgo.Some(witgo.Ok[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit](
			witgo.Err[witgo.Option[Trailers], ErrorCode](errorCode),
		))
	}

	var trailers witgo.Option[Trailers]
	if future.Result.Trailers != nil {
		trailers = witgo.Some(i.hm.Fields.Add(future.Result.Trailers))
	} else {
		trailers = witgo.None[Trailers]()
	}

	return witgo.Some(witgo.Ok[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit](
		witgo.Ok[witgo.Option[Trailers], ErrorCode](trailers),
	))
}
