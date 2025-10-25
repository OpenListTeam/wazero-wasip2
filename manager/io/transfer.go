package io

import (
	"io"
	"sync"
)

// Optimized buffer sizes for different transfer scenarios
const (
	MinBufferSize    = 8192    // 8KB minimum
	MaxBufferSize    = 1048576 // 1MB maximum
	DefaultChunkSize = 65536   // 64KB default chunk
)

// Global pool for large transfer buffers
var transferBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, MaxBufferSize)
		return &buf
	},
}

// GetTransferBuffer obtains a buffer from the pool
func GetTransferBuffer() *[]byte {
	return transferBufferPool.Get().(*[]byte)
}

// PutTransferBuffer returns a buffer to the pool
func PutTransferBuffer(buf *[]byte) {
	if buf != nil && len(*buf) == MaxBufferSize {
		transferBufferPool.Put(buf)
	}
}

// OptimizedCopy performs an optimized copy operation with buffer pooling
func OptimizedCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	bufPtr := GetTransferBuffer()
	defer PutTransferBuffer(bufPtr)
	
	return io.CopyBuffer(dst, src, *bufPtr)
}

// AdaptiveBuffer dynamically adjusts buffer size based on throughput
type AdaptiveBuffer struct {
	buf          []byte
	size         int
	minSize      int
	maxSize      int
	resizeAfter  int
	opsCount     int
	lastThroughput int64
}

// NewAdaptiveBuffer creates a buffer that adapts to workload
func NewAdaptiveBuffer(initialSize, minSize, maxSize int) *AdaptiveBuffer {
	if initialSize < minSize {
		initialSize = minSize
	}
	if initialSize > maxSize {
		initialSize = maxSize
	}
	
	return &AdaptiveBuffer{
		buf:         make([]byte, initialSize),
		size:        initialSize,
		minSize:     minSize,
		maxSize:     maxSize,
		resizeAfter: 10, // Evaluate after 10 operations
		opsCount:    0,
	}
}

// Buffer returns the current buffer
func (ab *AdaptiveBuffer) Buffer() []byte {
	return ab.buf
}

// Resize adjusts buffer size based on usage pattern
func (ab *AdaptiveBuffer) Resize(bytesProcessed int64) {
	ab.opsCount++
	
	if ab.opsCount < ab.resizeAfter {
		return
	}
	
	// Calculate average throughput per operation
	avgThroughput := bytesProcessed / int64(ab.opsCount)
	
	// Grow buffer if consistently processing large amounts
	if avgThroughput > int64(ab.size)*3/4 && ab.size < ab.maxSize {
		newSize := ab.size * 2
		if newSize > ab.maxSize {
			newSize = ab.maxSize
		}
		ab.buf = make([]byte, newSize)
		ab.size = newSize
		ab.opsCount = 0
	} else if avgThroughput < int64(ab.size)/4 && ab.size > ab.minSize {
		// Shrink buffer if underutilized
		newSize := ab.size / 2
		if newSize < ab.minSize {
			newSize = ab.minSize
		}
		ab.buf = make([]byte, newSize)
		ab.size = newSize
		ab.opsCount = 0
	}
	
	ab.lastThroughput = avgThroughput
}
