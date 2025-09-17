package v0_2

import (
	"wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type futureTrailersImpl struct {
	hm *http.HTTPManager
}

func newFutureTrailersImpl(hm *http.HTTPManager) *futureTrailersImpl {
	return &futureTrailersImpl{hm: hm}
}

func (i *futureTrailersImpl) Drop(this FutureTrailers) {
	i.hm.FutureTrailers.Remove(this)
}

func (i *futureTrailersImpl) Subscribe(this FutureTrailers) Pollable {
	f, ok := i.hm.FutureTrailers.Get(this)
	if !ok {
		return 0
	}
	f.PollableOnce.Do(func() {
		f.Pollable = make(chan struct{})
		go func() {
			select {
			case r := <-f.ResultChan:
				f.Result.Store(&r)
				close(f.Pollable)
			}
		}()
	})
	return i.hm.Poll.Add(f.Pollable)
}

func (i *futureTrailersImpl) Get(this FutureTrailers) witgo.Option[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]] {
	f, ok := i.hm.FutureTrailers.Get(this)
	if !ok {
		return witgo.None[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]]()
	}
	res := f.Result.Load()
	if res == nil {
		return witgo.None[witgo.Result[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit]]()
	}

	if f.Consumed.Swap(true) {
		// Already consumed
		return witgo.Some(witgo.Err[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit](witgo.Unit{}))
	}

	if res.Err != nil {
		// TODO: Convert Go error to ErrorCode
		return witgo.Some(witgo.Ok[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit](
			witgo.Err[witgo.Option[Trailers], ErrorCode](ErrorCode{InternalError: witgo.SomePtr(res.Err.Error())}),
		))
	}

	trailers := witgo.Some(res.TrailersHandle)
	return witgo.Some(witgo.Ok[witgo.Result[witgo.Option[Trailers], ErrorCode], witgo.Unit](
		witgo.Ok[witgo.Option[Trailers], ErrorCode](trailers),
	))
}
