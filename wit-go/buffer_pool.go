package witgo

import (
	"sync"
)

// BufferPool manages reusable byte slices to reduce allocation overhead
type BufferPool struct {
	pools map[int]*sync.Pool
	sizes []int
}

// NewBufferPool creates a new buffer pool with predefined size tiers
func NewBufferPool() *BufferPool {
	sizes := []int{
		1024,      // 1KB
		8192,      // 8KB
		32768,     // 32KB
		131072,    // 128KB
		524288,    // 512KB
		2097152,   // 2MB
		8388608,   // 8MB
	}

	pools := make(map[int]*sync.Pool)
	for _, size := range sizes {
		s := size // capture loop variable
		pools[size] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, s)
			},
		}
	}

	return &BufferPool{
		pools: pools,
		sizes: sizes,
	}
}

// Get returns a buffer of at least the requested size
func (bp *BufferPool) Get(size int) []byte {
	for _, poolSize := range bp.sizes {
		if poolSize >= size {
			return bp.pools[poolSize].Get().([]byte)[:size]
		}
	}
	// If requested size exceeds all pool sizes, allocate directly
	return make([]byte, size)
}

// Put returns a buffer to the pool for reuse
func (bp *BufferPool) Put(buf []byte) {
	capacity := cap(buf)
	for _, poolSize := range bp.sizes {
		if capacity == poolSize {
			// Reset the slice to full capacity before returning
			buf = buf[:cap(buf)]
			bp.pools[poolSize].Put(buf)
			return
		}
	}
	// Buffer doesn't match any pool size, let GC handle it
}

// Global buffer pool instance
var globalBufferPool = NewBufferPool()

// GetBuffer is a convenience function to get a buffer from the global pool
func GetBuffer(size int) []byte {
	return globalBufferPool.Get(size)
}

// PutBuffer is a convenience function to return a buffer to the global pool
func PutBuffer(buf []byte) {
	globalBufferPool.Put(buf)
}
