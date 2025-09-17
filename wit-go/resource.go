package witgo

import (
	"sync"
	"sync/atomic"
)

// ResourceManager[T] is a generic, thread-safe handle table for managing
// host-owned resources of a specific type T.
// T should be the concrete Go type of the resource (e.g., *MyStream, *os.File).
type ResourceManager[T any] struct {
	mu      sync.RWMutex
	handles map[uint32]T // The map now stores values of the generic type T.
	nextID  uint32
}

// NewResourceManager creates a new generic resource manager for a specific type.
func NewResourceManager[T any]() *ResourceManager[T] {
	return &ResourceManager[T]{
		handles: make(map[uint32]T),
		nextID:  0, // Start handles at 1 for safety (0 is often an invalid handle).
	}
}

// Add stores a new resource and returns a handle to it.
func (m *ResourceManager[T]) Add(resource T) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle := atomic.AddUint32(&m.nextID, 1)
	m.handles[handle] = resource
	return handle
}

// Get retrieves a resource by its handle. The returned value is already of type T.
func (m *ResourceManager[T]) Get(handle uint32) (T, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res, ok := m.handles[handle]
	return res, ok
}

// Remove deletes a resource by its handle, allowing it to be garbage collected.
func (m *ResourceManager[T]) Remove(handle uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.handles, handle)
}

// Range iterates over the resources in the manager.
// It calls the given function for each handle and resource. If the function
// returns false, the iteration stops.
func (m *ResourceManager[T]) Range(f func(handle uint32, resource T) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for handle, resource := range m.handles {
		if !f(handle, resource) {
			break
		}
	}
}
