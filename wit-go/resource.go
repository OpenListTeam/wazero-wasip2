package witgo

import (
	"sync"
)

// DestructorFunc defines the signature for a function that cleans up a resource.
type DestructorFunc[T any] func(resource T)

// ResourceManager[T] is a generic, thread-safe handle table for managing
// host-owned resources of a specific type T.
// T should be the concrete Go type of the resource (e.g., *MyStream, *os.File).
type ResourceManager[T any] struct {
	mu         sync.RWMutex
	handles    map[uint32]T
	nextID     uint32
	destructor DestructorFunc[T] // Optional function to call when a resource is removed.
}

// NewResourceManager creates a new generic resource manager for a specific type.
// The optional destructor function is called when a resource is removed via the
// Remove method. It is NOT called for Pop, which implies ownership transfer.
func NewResourceManager[T any](destructor DestructorFunc[T]) *ResourceManager[T] {
	return &ResourceManager[T]{
		handles:    make(map[uint32]T),
		nextID:     0, // Start handles at 1 for safety (0 is often an invalid handle).
		destructor: destructor,
	}
}

// Set associates a resource with a specific handle. If the handle is greater
// than the internal counter, the counter is updated.
func (m *ResourceManager[T]) Set(handle uint32, resource T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// IMPROVEMENT: Removed redundant atomic operations. The exclusive lock already
	// guarantees atomicity for this block.
	if m.nextID < handle {
		m.nextID = handle
	}
	m.handles[handle] = resource
}

// Add stores a new resource and returns a handle to it.
func (m *ResourceManager[T]) Add(resource T) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()

	// IMPROVEMENT: Replaced atomic operation with a simple increment. It's safe
	// due to the surrounding exclusive lock.
	m.nextID++
	handle := m.nextID
	m.handles[handle] = resource
	return handle
}

// Get retrieves a resource by its handle.
func (m *ResourceManager[T]) Get(handle uint32) (T, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res, ok := m.handles[handle]
	return res, ok
}

// Remove deletes a resource by its handle. If a destructor was provided on
// creation, it is called on the resource before it's removed.
func (m *ResourceManager[T]) Remove(handle uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First, find the resource that corresponds to the handle.
	res, ok := m.handles[handle]
	if !ok {
		return false // Nothing to do if handle doesn't exist.
	}

	// NEW: If a destructor function is set, call it with the resource.
	if m.destructor != nil {
		m.destructor(res)
	}

	delete(m.handles, handle)
	return true
}

// Pop retrieves and removes the resource by its handle in a single operation.
// The destructor is NOT called, as this method implies a transfer of ownership.
func (m *ResourceManager[T]) Pop(handle uint32) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.handles[handle]
	if ok {
		delete(m.handles, handle)
	}
	return res, ok
}

// Range iterates over the resources in the manager.
// It calls the given function for each handle and resource. If the function
// returns false, the iteration stops.
func (m *ResourceManager[T]) Range(f func(handle uint32, resource T) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Note: The iteration order over a map is not guaranteed in Go.
	for handle, resource := range m.handles {
		if !f(handle, resource) {
			break
		}
	}
}

// DoWith executes the provided function with the resource associated with
// the given handle. It returns true if the resource was found and the function
// was executed, false otherwise.
func (m *ResourceManager[T]) DoWith(handle uint32, do func(resource T)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	res, ok := m.handles[handle]
	if !ok {
		return false
	}
	do(res)
	return true
}

// CloseAll removes all resources and calls destructor on each if provided
// This is useful for cleanup when shutting down a Host instance
func (m *ResourceManager[T]) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.destructor != nil {
		for _, resource := range m.handles {
			m.destructor(resource)
		}
	}

	// Clear the map
	m.handles = make(map[uint32]T)
}
