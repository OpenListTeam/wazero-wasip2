package wasip2

import (
	"context"
	"wazero-wasip2/internal/errors"
	"wazero-wasip2/internal/http"
	"wazero-wasip2/internal/poll"
	"wazero-wasip2/internal/streams"

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
	streamManager *streams.Manager
	errorManager  *errors.ResourceManager
	pollManager   *poll.Manager
	httpManager   *http.HTTPManager
	// 未来可以在这里添加 httpManager 等其他状态管理器

	implementations []Implementation
}

// ModuleOption 是用于配置 Host 的选项函数。
type ModuleOption func(*Host)

// NewHost 创建一个新的 Host 实例，并应用所有提供的模块选项。
func NewHost(opts ...ModuleOption) *Host {
	streamManager := streams.NewManager()
	pollManager := poll.NewManager()
	h := &Host{
		streamManager: streamManager,
		errorManager:  errors.NewManager(),
		pollManager:   pollManager,
		httpManager:   http.NewHTTPManager(streamManager, pollManager),
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

func (h *Host) StreamManager() *streams.Manager {
	return h.streamManager
}

func (h *Host) ErrorManager() *errors.ResourceManager {
	return h.errorManager
}

func (h *Host) PollManager() *poll.Manager {
	return h.pollManager
}

func (h *Host) HTTPManager() *http.HTTPManager {
	return h.httpManager
}
