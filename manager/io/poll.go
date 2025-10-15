package io

import (
	"sync"

	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type IPollable interface {
	// IsReady 以非阻塞方式检查 Pollable 是否就绪。
	IsReady() bool
	// Block 阻塞直到 Pollable 就绪。
	Block()
	// Channel 返回内部的 channel，用于 reflect.Select。
	Channel() <-chan struct{}
	// Close 用于释放与 Pollable 关联的资源，例如取消底层的定时器。
	Close()
}

// channelPollable 是 IPollable 接口的一个具体实现，它使用 channel 来进行阻塞。
// 这个实现是线程安全的。
type ChannelPollable struct {
	mu        sync.Mutex
	readyChan chan struct{}
	cancel    func() // 用于 Close()
}

// NewPollable 创建一个新的 channelPollable 实例。
func NewPollable(cancel func()) *ChannelPollable {
	return &ChannelPollable{
		readyChan: make(chan struct{}),
		cancel:    cancel,
	}
}

func NewPollableByChan(c chan struct{}, cancel func()) *ChannelPollable {
	return &ChannelPollable{
		readyChan: c,
		cancel:    cancel,
	}
}

var ReadyPollable = NewReadyPollable()

// NewReadyPollable 创建一个已经处于“就绪”状态的 ChannelPollable。
func NewReadyPollable() *ChannelPollable {
	ch := make(chan struct{})
	close(ch)
	return &ChannelPollable{
		readyChan: ch,
	}
}

// IsReady 以非阻塞方式检查 Pollable 是否就绪。
func (p *ChannelPollable) IsReady() bool {
	p.mu.Lock()
	ch := p.readyChan
	p.mu.Unlock()

	select {
	case <-ch:
		return true // channel 已关闭，代表“就绪”
	default:
		return false // channel 是打开的，代表“未就绪”
	}
}

// Block 阻塞直到 Pollable 就绪。
func (p *ChannelPollable) Block() {
	p.mu.Lock()
	ch := p.readyChan
	p.mu.Unlock()

	<-ch // 阻塞直到 channel 关闭
}

// SetReady 将 Pollable 状态设置为就绪。这个操作是幂等的。
// 如果已经就绪，它不会做任何事情。
func (p *ChannelPollable) SetReady() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查 channel 是否已经关闭，以避免 panic
	select {
	case <-p.readyChan:
		// 已经关闭（已就绪），什么都不用做。
		return
	default:
		// 尚未关闭，关闭它以将其设置为“就绪”。
		close(p.readyChan)
	}
}

// Reset 将 Pollable 重置为“未就绪”状态，使其可以被再次使用。
// 如果已经处于“未就绪”状态，它不会做任何事情。
func (p *ChannelPollable) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.readyChan:
		// channel 已关闭（已就绪），因此我们需要创建一个新的 channel。
		p.readyChan = make(chan struct{})
	default:
		// channel 仍然是打开的（未就绪），无需任何操作。
	}
}

// Channel 返回当前的内部 channel。
// 警告：返回的 channel 可能会在 Reset 调用后失效。
// 主要用于 select 语句。
func (p *ChannelPollable) Channel() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readyChan
}

// Close 调用与此 pollable 关联的取消函数（如果存在）。
func (p *ChannelPollable) Close() {
	if p.cancel != nil {
		p.cancel()
	}
}

// Manager 是用于管理所有 Pollable 资源的管理器。
type PollManager = witgo.ResourceManager[IPollable]

// NewManager 创建一个新的 Poll 管理器。
func NewPollManager() *PollManager {
	return witgo.NewResourceManager[IPollable](func(resource IPollable) {
		if resource.Close != nil {
			resource.Close()
		}
	})
}
