package io

import (
	"sync"
	"sync/atomic"
	witgo "wazero-wasip2/wit-go"
)

type IPollable interface {
	// IsReady 以非阻塞方式检查 Pollable 是否就绪。
	IsReady() bool
	// Block 阻塞直到 Pollable 就绪。
	Block()
	// Channel 返回内部的 channel，用于 reflect.Select。
	Channel() chan struct{}
	// Close 用于释放与 Pollable 关联的资源，例如取消底层的定时器。
	Close()
}

// channelPollable 是 IPollable 接口的一个具体实现，它使用 channel 来进行阻塞。
type ChannelPollable struct {
	readyChan atomic.Value
	cancel    func()
	once      sync.Once
}

// NewPollable 创建一个新的 channelPollable 实例。
func NewPollable(cancel func()) *ChannelPollable {
	v := atomic.Value{}
	v.Store(make(chan struct{}))
	return &ChannelPollable{
		readyChan: v,
		cancel:    cancel,
	}
}

func NewPollableByChan(c chan struct{}, cancel func()) *ChannelPollable {
	v := atomic.Value{}
	v.Store(c)
	return &ChannelPollable{
		readyChan: v,
		cancel:    cancel,
	}
}

var ReadyPollable = NewReadyPollable()

func NewReadyPollable() *ChannelPollable {
	c := NewPollable(nil)
	c.SetReady()
	return c
}

// IsReady 以非阻塞方式检查 Pollable 是否就绪。
func (p *ChannelPollable) IsReady() bool {
	select {
	case <-p.readyChan.Load().(chan struct{}):
		return true
	default:
		return false
	}
}

// Block 阻塞直到 Pollable 就绪。
func (p *ChannelPollable) Block() {
	<-p.readyChan.Load().(chan struct{})
}

// SetReady 将 Pollable 状态设置为就绪。这个操作是幂等的。
func (p *ChannelPollable) SetReady() {
	p.once.Do(func() {
		close(p.readyChan.Load().(chan struct{}))
	})
}

// Channel 返回内部的 channel，用于 reflect.Select。
func (p *ChannelPollable) Channel() chan struct{} {
	return p.readyChan.Load().(chan struct{})
}

// Close 调用与此 pollable 关联的取消函数（如果存在）。
func (p *ChannelPollable) Close() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *ChannelPollable) Reset() {
	c := p.readyChan.Swap(make(chan struct{})).(chan struct{})
	select {
	case <-c:
	default:
		close(c)
	}
}

// Manager 是用于管理所有 Pollable 资源的管理器。
type PollManager = witgo.ResourceManager[IPollable]

// NewManager 创建一个新的 Poll 管理器。
func NewPollManager() *PollManager {
	return witgo.NewResourceManager[IPollable]()
}
