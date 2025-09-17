package v0_2

import (
	"context"
	"wazero-wasip2/internal/sockets"
	"wazero-wasip2/wasip2"
)

type networkImpl struct {
	host *wasip2.Host
}

func newNetworkImpl(h *wasip2.Host) *networkImpl {
	return &networkImpl{host: h}
}

// DropNetwork 是 network 资源的析构函数。
func (i *networkImpl) DropNetwork(_ context.Context, handle Network) {
	i.host.NetworkManager().Remove(handle)
}

// InstanceNetwork 返回一个代表默认网络访问能力的句柄。
func (i *networkImpl) InstanceNetwork(_ context.Context) Network {
	// TODO
	// 在一个真实的实现中，这里可能会有复杂的逻辑来确定
	// “默认网络”是什么，以及 Guest 是否有权限访问它。
	// 在我们的实现中，我们简单地创建一个新的 Network 资源。
	net := &sockets.Network{}
	return i.host.NetworkManager().Add(net)
}
