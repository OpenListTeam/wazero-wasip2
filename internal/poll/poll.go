package poll

import witgo "wazero-wasip2/wit-go"

// Pollable 在 Host 端的表示。我们使用一个 channel 来实现“就绪”状态的通知。
// 一个 struct{} 类型的 channel 是最轻量的，因为我们只关心它的关闭事件。
type Pollable = chan struct{}

// Manager 是用于管理所有 Pollable 资源的管理器。
type Manager = witgo.ResourceManager[Pollable]

// NewManager 创建一个新的 Poll 管理器。
func NewManager() *Manager {
	return witgo.NewResourceManager[Pollable]()
}
