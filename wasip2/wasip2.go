package wasip2

import (
	"context"

	"github.com/OpenListTeam/wazero-wasip2/manager/filesystem"
	"github.com/OpenListTeam/wazero-wasip2/manager/http"
	"github.com/OpenListTeam/wazero-wasip2/manager/io"
	"github.com/OpenListTeam/wazero-wasip2/manager/sockets"
	"github.com/OpenListTeam/wazero-wasip2/manager/tls"

	"github.com/tetratelabs/wazero"
)

// Implementation 是所有 WASI 模块必须实现的接口。
type Implementation interface {
	// Name 返回模块的名称，例如 "wasi:io"。
	Name() string
	// Versions 返回此实现兼容的 WIT 版本列表，例如 ["0.2.0", "0.2.1"]。
	Versions() []string
	// Instantiate 将模块的函数导出到 wazero 运行时。
	Instantiate(context.Context, *Host, wazero.HostModuleBuilder) error
}

// Host 是所有 WASI 实现的容器。
type Host struct {
	streamManager *io.StreamManager
	errorManager  *io.ErrorManager
	pollManager   *io.PollManager
	httpManager   *http.HTTPManager
	tlsManager    *tls.TLSManager

	// filesystem 管理器
	filesystemManager           *filesystem.Manager
	directoryEntryStreamManager *filesystem.DirectoryEntryStreamManager

	//  Sockets 管理器
	networkManager              *sockets.NetworkManager
	tcpSocketManager            *sockets.TCPSocketManager
	udpSocketManager            *sockets.UDPSocketManager
	resolveAddressStreamManager *sockets.ResolveAddressStreamManager
	// 未来可以在这里添加 httpManager 等其他状态管理器

	implementations []Implementation
}

// ModuleOption 是用于配置 Host 的选项函数。
type ModuleOption func(*Host)

// NewHost 创建一个新的 Host 实例，并应用所有提供的模块选项。
func NewHost(opts ...ModuleOption) *Host {
	streamManager, pollManager, errorManager := io.NewManager()
	h := &Host{
		streamManager: streamManager,
		errorManager:  errorManager,
		pollManager:   pollManager,
		httpManager:   http.NewHTTPManager(streamManager, pollManager),
		tlsManager:    tls.NewTLSManager(),

		filesystemManager:           filesystem.NewManager(),
		directoryEntryStreamManager: filesystem.NewDirectoryEntryStreamManager(),

		networkManager:              sockets.NewNetworkManager(),
		tcpSocketManager:            sockets.NewTCPSocketManager(),
		udpSocketManager:            sockets.NewUDPSocketManager(),
		resolveAddressStreamManager: sockets.NewResolveAddressStreamManager(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *Host) AddImplementation(impl Implementation) {
	h.implementations = append(h.implementations, impl)
}

// Instantiate 将所有已配置的模块实例化到 wazero 运行时。
func (h *Host) Instantiate(ctx context.Context, r wazero.Runtime) error {
	for _, impl := range h.implementations {
		for _, version := range impl.Versions() {
			moduleName := impl.Name() + "@" + version
			builder := r.NewHostModuleBuilder(moduleName)
			if err := impl.Instantiate(ctx, h, builder); err != nil {
				return err
			}

			if _, err := builder.Instantiate(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Host) StreamManager() *io.StreamManager {
	return h.streamManager
}

func (h *Host) ErrorManager() *io.ErrorManager {
	return h.errorManager
}

func (h *Host) PollManager() *io.PollManager {
	return h.pollManager
}

func (h *Host) HTTPManager() *http.HTTPManager {
	return h.httpManager
}

// FilesystemManager 返回文件系统管理器。
func (h *Host) FilesystemManager() *filesystem.Manager {
	return h.filesystemManager
}

// DirectoryEntryStreamManager 返回目录条目流管理器。
func (h *Host) DirectoryEntryStreamManager() *filesystem.DirectoryEntryStreamManager {
	return h.directoryEntryStreamManager
}

func (h *Host) NetworkManager() *sockets.NetworkManager {
	return h.networkManager
}
func (h *Host) TCPSocketManager() *sockets.TCPSocketManager {
	return h.tcpSocketManager
}
func (h *Host) UDPSocketManager() *sockets.UDPSocketManager {
	return h.udpSocketManager
}
func (h *Host) ResolveAddressStreamManager() *sockets.ResolveAddressStreamManager {
	return h.resolveAddressStreamManager
}

func (h *Host) TLSManager() *tls.TLSManager {
	return h.tlsManager
}

// Close releases all resources managed by this Host instance
// This should be called when the Host is no longer needed to prevent resource leaks
func (h *Host) Close() error {
	// Close all managers in reverse order of creation to handle dependencies
	h.resolveAddressStreamManager.CloseAll()
	h.udpSocketManager.CloseAll()
	h.tcpSocketManager.CloseAll()
	h.networkManager.CloseAll()

	h.directoryEntryStreamManager.CloseAll()
	h.filesystemManager.CloseAll()

	h.tlsManager.ClientHandshakes.CloseAll()
	h.tlsManager.ClientConnections.CloseAll()
	h.tlsManager.FutureClientStreams.CloseAll()

	h.httpManager.IncomingBodies.CloseAll()
	h.httpManager.OutgoingResponses.CloseAll()
	h.httpManager.ResponseOutparams.CloseAll()
	h.httpManager.IncomingRequests.CloseAll()
	h.httpManager.FutureTrailers.CloseAll()
	h.httpManager.Bodies.CloseAll()
	h.httpManager.Responses.CloseAll()
	h.httpManager.Futures.CloseAll()
	h.httpManager.OutgoingRequests.CloseAll()
	h.httpManager.Options.CloseAll()
	h.httpManager.Fields.CloseAll()

	h.pollManager.CloseAll()
	h.errorManager.CloseAll()
	h.streamManager.CloseAll()

	return nil
}
