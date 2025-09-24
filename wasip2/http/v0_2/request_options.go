package v0_2

import (
	"context"
	"time"
	manager_http "wazero-wasip2/internal/http"
	witgo "wazero-wasip2/wit-go"
)

type requestOptionsImpl struct {
	hm *manager_http.HTTPManager
}

func newRequestOptionsImpl(hm *manager_http.HTTPManager) *requestOptionsImpl {
	return &requestOptionsImpl{hm: hm}
}

func (i *requestOptionsImpl) Constructor() RequestOptions {
	return i.hm.Options.Add(&manager_http.RequestOptions{})
}

func (i *requestOptionsImpl) Drop(_ context.Context, handle RequestOptions) {
	i.hm.Options.Remove(handle)
}

// --- Getters ---
func (i *requestOptionsImpl) ConnectTimeout(_ context.Context, this RequestOptions) witgo.Option[Duration] {
	opts, ok := i.hm.Options.Get(this)
	if !ok || opts.ConnectTimeout == nil {
		return witgo.None[Duration]()
	}
	return witgo.Some(Duration(*opts.ConnectTimeout))
}

func (i *requestOptionsImpl) FirstByteTimeout(_ context.Context, this RequestOptions) witgo.Option[Duration] {
	opts, ok := i.hm.Options.Get(this)
	if !ok || opts.FirstByteTimeout == nil {
		return witgo.None[Duration]()
	}
	return witgo.Some(Duration(*opts.FirstByteTimeout))
}

func (i *requestOptionsImpl) BetweenBytesTimeout(_ context.Context, this RequestOptions) witgo.Option[Duration] {
	opts, ok := i.hm.Options.Get(this)
	if !ok || opts.BetweenBytesTimeout == nil {
		return witgo.None[Duration]()
	}
	return witgo.Some(Duration(*opts.BetweenBytesTimeout))
}

// --- Setters ---
func (i *requestOptionsImpl) SetConnectTimeout(_ context.Context, this RequestOptions, duration witgo.Option[Duration]) witgo.UnitResult {
	opts, ok := i.hm.Options.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	if duration.Some != nil {
		d := time.Duration(*duration.Some)
		opts.ConnectTimeout = &d
	} else {
		opts.ConnectTimeout = nil
	}
	return witgo.UintOk()
}

func (i *requestOptionsImpl) SetFirstByteTimeout(_ context.Context, this RequestOptions, duration witgo.Option[Duration]) witgo.UnitResult {
	opts, ok := i.hm.Options.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	if duration.Some != nil {
		d := time.Duration(*duration.Some)
		opts.FirstByteTimeout = &d
	} else {
		opts.FirstByteTimeout = nil
	}
	return witgo.UintOk()
}

func (i *requestOptionsImpl) SetBetweenBytesTimeout(_ context.Context, this RequestOptions, duration witgo.Option[Duration]) witgo.UnitResult {
	opts, ok := i.hm.Options.Get(this)
	if !ok {
		return witgo.UintErr()
	}
	if duration.Some != nil {
		d := time.Duration(*duration.Some)
		opts.BetweenBytesTimeout = &d
	} else {
		opts.BetweenBytesTimeout = nil
	}
	return witgo.UintOk()
}
