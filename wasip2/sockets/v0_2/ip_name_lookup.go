package v0_2

import (
	"context"
	"net"
	manager_io "wazero-wasip2/internal/io"
	"wazero-wasip2/internal/sockets"
	"wazero-wasip2/wasip2"
	wasip2_io "wazero-wasip2/wasip2/io/v0_2"
	witgo "wazero-wasip2/wit-go"
)

type ipNameLookupImpl struct {
	host *wasip2.Host
}

func newIPNameLookupImpl(h *wasip2.Host) *ipNameLookupImpl {
	return &ipNameLookupImpl{host: h}
}

func (i *ipNameLookupImpl) ResolveAddresses(_ context.Context, network Network, name string) witgo.Result[ResolveAddressStream, ErrorCode] {
	// 创建一个用于管理此解析操作状态的资源
	state := &sockets.ResolveAddressStreamState{
		Done: make(chan struct{}),
	}
	handle := i.host.ResolveAddressStreamManager().Add(state)

	// 在后台 goroutine 中执行阻塞的 DNS 查询
	go func() {
		defer close(state.Done)
		addrs, err := net.LookupIP(name)
		if err != nil {
			state.Error = err
			return
		}
		state.Addresses = addrs
	}()

	return witgo.Ok[ResolveAddressStream, ErrorCode](handle)
}

func (i *ipNameLookupImpl) DropResolveAddressStream(_ context.Context, handle ResolveAddressStream) {
	i.host.ResolveAddressStreamManager().Remove(handle)
}

func (i *ipNameLookupImpl) ResolveNextAddress(_ context.Context, this ResolveAddressStream) witgo.Result[witgo.Option[IPAddress], ErrorCode] {
	state, ok := i.host.ResolveAddressStreamManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Option[IPAddress], ErrorCode](ErrorCodeInvalidArgument)
	}

	// 检查后台任务是否已完成
	select {
	case <-state.Done:
		// 已完成
		if state.Error != nil {
			// 如果解析出错，返回错误
			// TODO: 更精细的错误映射
			return witgo.Err[witgo.Option[IPAddress], ErrorCode](ErrorCodeNameUnresolvable)
		}

		if state.Index >= len(state.Addresses) {
			// 所有地址都已返回
			return witgo.Ok[witgo.Option[IPAddress], ErrorCode](witgo.None[IPAddress]())
		}

		// 返回下一个地址
		addr := state.Addresses[state.Index]
		state.Index++

		wasiAddr, err := toIPAddress(addr)
		if err != nil {
			// 如果地址转换失败，尝试下一个
			return i.ResolveNextAddress(context.Background(), this)
		}

		return witgo.Ok[witgo.Option[IPAddress], ErrorCode](witgo.Some(wasiAddr))

	default:
		// 尚未完成
		return witgo.Err[witgo.Option[IPAddress], ErrorCode](ErrorCodeWouldBlock)
	}
}

func (i *ipNameLookupImpl) Subscribe(_ context.Context, this ResolveAddressStream) wasip2_io.Pollable {
	state, ok := i.host.ResolveAddressStreamManager().Get(this)
	if !ok {
		p := manager_io.NewPollable(nil)
		close(p.ReadyChan) // 无效句柄立即就绪
		return i.host.PollManager().Add(p)
	}

	p := manager_io.NewPollable(nil) // 创建一个新的 pollable
	handle := i.host.PollManager().Add(p)

	// 启动一个 goroutine 等待解析完成，然后关闭 pollable 的 channel
	go func() {
		<-state.Done
		close(p.ReadyChan)
	}()

	return handle
}
