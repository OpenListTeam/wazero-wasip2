package io

import (
	witgo "wazero-wasip2/wit-go"
)

// Pollable 代表一个可轮询的 I/O 事件状态。
// 它使用一个 channel 来进行阻塞，并用 atomic.Value 来实现对 channel 的无锁读取。
type Pollable struct {
	// ReadyChan 在事件就绪时关闭。
	ReadyChan chan struct{}
	// Cancel 用于在 Guest drop 句柄时，取消底层的异步操作（如定时器）。
	Cancel func()
}

// NewPollable 创建一个新的 Pollable 实例。
func NewPollable(cancel func()) *Pollable {
	return &Pollable{
		ReadyChan: make(chan struct{}),
		Cancel:    cancel,
	}

}

// IsReady 以非阻塞方式检查 Pollable 是否就绪。
func (p *Pollable) IsReady() bool {
	select {
	case <-p.ReadyChan:
		return true
	default:
		return false
	}
}

// Block 阻塞直到 Pollable 就绪。
func (p *Pollable) Block() {
	<-p.ReadyChan
}

// SetReady 将 Pollable 状态设置为就绪。
func (p *Pollable) SetReady() {
	select {
	case <-p.ReadyChan:
	default:
		close(p.ReadyChan)
	}
}

// Channel 返回内部的 channel，用于 reflect.Select。
func (p *Pollable) Channel() chan struct{} {
	return p.ReadyChan
}

// Manager 是用于管理所有 Pollable 资源的管理器。
type PollManager = witgo.ResourceManager[*Pollable]

// NewManager 创建一个新的 Poll 管理器。
func NewPollManager() *PollManager {
	return witgo.NewResourceManager[*Pollable]()
}
