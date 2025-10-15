package filesystem

import (
	"io/fs"
	"os"

	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

// Descriptor represents a file or directory descriptor.
// It holds the underlying os.File and the pre-opened path.
type Descriptor struct {
	// The underlying file descriptor from Go's os package.
	File *os.File
	// The path this descriptor was pre-opened with, for identification.
	Path string
	// Permissions associated with this descriptor.
	Flags fs.FileMode // We can use fs.FileMode to store basic permissions
}

// Manager is the resource manager for all filesystem descriptors.
type Manager = witgo.ResourceManager[*Descriptor]

// NewManager creates a new filesystem descriptor manager.
func NewManager() *Manager {
	return witgo.NewResourceManager[*Descriptor](func(resource *Descriptor) {
		resource.File.Close()
	})
}

// DirectoryEntryStreamState 用于管理读取目录的状态。
type DirectoryEntryStreamState struct {
	// 预读的目录条目
	Entries []fs.DirEntry
	// 当前读取到的索引位置
	Index int
}

// DirectoryEntryStreamManager 是用于管理目录条目流的资源管理器。
type DirectoryEntryStreamManager = witgo.ResourceManager[*DirectoryEntryStreamState]

// NewDirectoryEntryStreamManager 创建一个新的目录流管理器。
func NewDirectoryEntryStreamManager() *DirectoryEntryStreamManager {
	return witgo.NewResourceManager[*DirectoryEntryStreamState](nil)
}
