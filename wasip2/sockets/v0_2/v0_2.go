package v0_2

import (
	"context"

	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:sockets/network ---
type wasiNetwork struct{}

func NewNetwork() wasip2.Implementation {
	return &wasiNetwork{}
}
func (i *wasiNetwork) Name() string { return "wasi:sockets/network" }
func (i *wasiNetwork) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiNetwork) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newNetworkImpl(h)
	exporter := witgo.NewExporter(b)
	exporter.Export("[resource-drop]network", handler.DropNetwork)
	return nil
}

// --- wasi:sockets/instance-network ---
type wasiInstanceNetwork struct{}

func NewInstanceNetwork() wasip2.Implementation {
	return &wasiInstanceNetwork{}
}
func (i *wasiInstanceNetwork) Name() string { return "wasi:sockets/instance-network" }
func (i *wasiInstanceNetwork) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiInstanceNetwork) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newNetworkImpl(h)
	exporter := witgo.NewExporter(b)
	exporter.Export("instance-network", handler.InstanceNetwork)
	return nil
}

// --- wasi:sockets/tcp ---
type wasiTCP struct{}

func NewTCP() wasip2.Implementation {
	return &wasiTCP{}
}
func (i *wasiTCP) Name() string { return "wasi:sockets/tcp" }
func (i *wasiTCP) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiTCP) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newTCPImpl(h)
	exporter := witgo.NewExporter(b)

	exporter.Export("[resource-drop]tcp-socket", handler.DropTCPSocket)
	exporter.Export("[method]tcp-socket.start-bind", handler.StartBind)
	exporter.Export("[method]tcp-socket.finish-bind", handler.FinishBind)
	exporter.Export("[method]tcp-socket.start-connect", handler.StartConnect)
	exporter.Export("[method]tcp-socket.finish-connect", handler.FinishConnect)
	exporter.Export("[method]tcp-socket.start-listen", handler.StartListen)
	exporter.Export("[method]tcp-socket.finish-listen", handler.FinishListen)
	exporter.Export("[method]tcp-socket.accept", handler.Accept)
	exporter.Export("[method]tcp-socket.local-address", handler.LocalAddress)
	exporter.Export("[method]tcp-socket.remote-address", handler.RemoteAddress)
	exporter.Export("[method]tcp-socket.is-listening", handler.IsListening)
	exporter.Export("[method]tcp-socket.address-family", handler.AddressFamily)
	exporter.Export("[method]tcp-socket.set-listen-backlog-size", handler.SetListenBacklogSize)
	exporter.Export("[method]tcp-socket.keep-alive-enabled", handler.KeepAliveEnabled)
	exporter.Export("[method]tcp-socket.set-keep-alive-enabled", handler.SetKeepAliveEnabled)
	exporter.Export("[method]tcp-socket.keep-alive-idle-time", handler.KeepAliveIdleTime)
	exporter.Export("[method]tcp-socket.set-keep-alive-idle-time", handler.SetKeepAliveIdleTime)
	exporter.Export("[method]tcp-socket.keep-alive-interval", handler.KeepAliveInterval)
	exporter.Export("[method]tcp-socket.set-keep-alive-interval", handler.SetKeepAliveInterval)
	exporter.Export("[method]tcp-socket.keep-alive-count", handler.KeepAliveCount)
	exporter.Export("[method]tcp-socket.set-keep-alive-count", handler.SetKeepAliveCount)
	exporter.Export("[method]tcp-socket.hop-limit", handler.HopLimit)
	exporter.Export("[method]tcp-socket.set-hop-limit", handler.SetHopLimit)
	exporter.Export("[method]tcp-socket.receive-buffer-size", handler.ReceiveBufferSize)
	exporter.Export("[method]tcp-socket.set-receive-buffer-size", handler.SetReceiveBufferSize)
	exporter.Export("[method]tcp-socket.send-buffer-size", handler.SendBufferSize)
	exporter.Export("[method]tcp-socket.set-send-buffer-size", handler.SetSendBufferSize)
	exporter.Export("[method]tcp-socket.subscribe", handler.Subscribe)
	exporter.Export("[method]tcp-socket.shutdown", handler.Shutdown)

	return nil
}

// --- wasi:sockets/tcp-create-socket ---
type wasiTCPCreateSocket struct{}

func NewTCPCreateSocket() wasip2.Implementation {
	return &wasiTCPCreateSocket{}
}
func (i *wasiTCPCreateSocket) Name() string { return "wasi:sockets/tcp-create-socket" }
func (i *wasiTCPCreateSocket) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiTCPCreateSocket) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newTCPCreateSocketImpl(h)
	exporter := witgo.NewExporter(b)
	exporter.Export("create-tcp-socket", handler.CreateTCPSocket)
	return nil
}

// --- wasi:sockets/udp ---
type wasiUDP struct{}

func NewUDP() wasip2.Implementation {
	return &wasiUDP{}
}
func (i *wasiUDP) Name() string { return "wasi:sockets/udp" }
func (i *wasiUDP) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiUDP) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newUDPImpl(h)
	exporter := witgo.NewExporter(b)

	// 导出 udp-socket 资源的所有方法
	exporter.Export("[resource-drop]udp-socket", handler.DropUDPSocket)
	exporter.Export("[method]udp-socket.start-bind", handler.StartBind)
	exporter.Export("[method]udp-socket.finish-bind", handler.FinishBind)
	exporter.Export("[method]udp-socket.stream", handler.Stream)
	exporter.Export("[method]udp-socket.local-address", handler.LocalAddress)
	exporter.Export("[method]udp-socket.remote-address", handler.RemoteAddress)
	exporter.Export("[method]udp-socket.address-family", handler.AddressFamily)
	exporter.Export("[method]udp-socket.unicast-hop-limit", handler.UnicastHopLimit)
	exporter.Export("[method]udp-socket.set-unicast-hop-limit", handler.SetUnicastHopLimit)
	exporter.Export("[method]udp-socket.receive-buffer-size", handler.ReceiveBufferSize)
	exporter.Export("[method]udp-socket.set-receive-buffer-size", handler.SetReceiveBufferSize)
	exporter.Export("[method]udp-socket.send-buffer-size", handler.SendBufferSize)
	exporter.Export("[method]udp-socket.set-send-buffer-size", handler.SetSendBufferSize)
	exporter.Export("[method]udp-socket.subscribe", handler.Subscribe)

	// 导出 datagram-stream 资源的方法
	exporter.Export("[resource-drop]incoming-datagram-stream", handler.DropIncomingDatagramStream)
	exporter.Export("[method]incoming-datagram-stream.receive", handler.Receive)
	exporter.Export("[method]incoming-datagram-stream.subscribe", handler.SubscribeIncoming)

	exporter.Export("[resource-drop]outgoing-datagram-stream", handler.DropOutgoingDatagramStream)
	exporter.Export("[method]outgoing-datagram-stream.check-send", handler.CheckSend)
	exporter.Export("[method]outgoing-datagram-stream.send", handler.Send)
	exporter.Export("[method]outgoing-datagram-stream.subscribe", handler.SubscribeOutgoing)

	return nil
}

// --- wasi:sockets/udp-create-socket ---
type wasiUDPCreateSocket struct{}

func NewUDPCreateSocket() wasip2.Implementation {
	return &wasiUDPCreateSocket{}
}
func (i *wasiUDPCreateSocket) Name() string { return "wasi:sockets/udp-create-socket" }
func (i *wasiUDPCreateSocket) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiUDPCreateSocket) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newUDPCreateSocketImpl(h)
	exporter := witgo.NewExporter(b)
	exporter.Export("create-udp-socket", handler.CreateUDPSocket)
	return nil
}

// --- wasi:sockets/ip-name-lookup ---
type wasiIPNameLookup struct{}

func NewIPNameLookup() wasip2.Implementation {
	return &wasiIPNameLookup{}
}
func (i *wasiIPNameLookup) Name() string { return "wasi:sockets/ip-name-lookup" }
func (i *wasiIPNameLookup) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}
func (i *wasiIPNameLookup) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newIPNameLookupImpl(h)
	exporter := witgo.NewExporter(b)
	exporter.Export("resolve-addresses", handler.ResolveAddresses)
	exporter.Export("[resource-drop]resolve-address-stream", handler.DropResolveAddressStream)
	exporter.Export("[method]resolve-address-stream.resolve-next-address", handler.ResolveNextAddress)
	exporter.Export("[method]resolve-address-stream.subscribe", handler.Subscribe)
	return nil
}
