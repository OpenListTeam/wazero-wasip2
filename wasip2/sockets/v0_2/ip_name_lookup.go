package v0_2

import (
	"context"
	"net"

	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	wasip2_io "github.com/OpenListTeam/wazero-wasip2/wasip2/io/v0_2"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type ipNameLookupImpl struct {
	host *wasip2.Host
}

func newIPNameLookupImpl(h *wasip2.Host) *ipNameLookupImpl {
	return &ipNameLookupImpl{host: h}
}

func (i *ipNameLookupImpl) ResolveAddresses(ctx context.Context, network Network, name string) witgo.Result[ResolveAddressStream, ErrorCode] {
	// 创建一个用于管理此解析操作状态的资源
	state := &sockets.ResolveAddressStreamState{
		Done: make(chan struct{}),
	}
	handle := i.host.ResolveAddressStreamManager().Add(state)

	// 优先尝试将 `name` 解析为 IP 地址
	if ip := net.ParseIP(name); ip != nil {
		// 如果成功，直接设置结果，无需异步查询
		state.Addresses = []net.IP{ip}
		state.CloseDone() // 标记为已完成，使用 CloseDone 确保安全关闭
		return witgo.Ok[ResolveAddressStream, ErrorCode](handle)
	}

	// 如果不是 IP 地址，则在后台 goroutine 中执行 DNS 查询
	go func() {
		defer state.CloseDone()
		// 使用带 context 的 Resolver 以支持取消
		resolver := net.Resolver{}
		addrs, err := resolver.LookupIPAddr(context.Background(), name)
		if err != nil {
			state.Error = err
			return
		}
		// 将 net.IPAddr 转换为 net.IP
		state.Addresses = make([]net.IP, len(addrs))
		for i, ipAddr := range addrs {
			state.Addresses[i] = ipAddr.IP
		}
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
			return witgo.Err[witgo.Option[IPAddress], ErrorCode](mapDnsError(state.Error))
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
			// 如果地址转换失败 (例如，是不支持的类型), 递归尝试下一个
			// TODO：为避免无限循环，一个更健壮的实现可能会在这里增加一些保护机制
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
		// 如果句柄无效，返回一个立即就绪的 pollable
		return i.host.PollManager().Add(manager_io.ReadyPollable)
	}

	p := manager_io.NewPollableByChan(state.Done, nil)
	handle := i.host.PollManager().Add(p)
	return handle
}
