package io

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
)

const defaultBufferSize = 8192 // 默认缓冲区大小为 8KB

// --- Asynchronous Reader ---

// AsyncReadWrapperOption 是用于配置 AsyncReadWrapper 的函数类型。
type AsyncReadWrapperOption func(*AsyncReadWrapper)

// DontCloseReader 是一个选项，用于阻止在关闭 wrapper 时关闭底层的 reader。
func DontCloseReader() AsyncReadWrapperOption {
	return func(arw *AsyncReadWrapper) {
		arw.closeUnderlying = false
	}
}

// AsyncReadWrapper 将一个阻塞的 io.Reader 封装成一个非阻塞的 reader，
// 并通过 IPollable 接口提供就绪状态通知。
type AsyncReadWrapper struct {
	reader          io.Reader
	buffer          *bytes.Buffer
	mutex           sync.Mutex
	ready           *ChannelPollable
	done            chan struct{}
	err             error
	once            sync.Once
	closeUnderlying bool // 新增字段：是否关闭底层 reader
}

// NewAsyncReadWrapper 创建并启动一个新的异步读取封装器。
// 默认情况下，它会在 Close 时关闭底层的 reader（如果它实现了 io.Closer）。
// 可以使用 DontCloseReader() 选项来改变这个行为。
func NewAsyncReadWrapper(r io.Reader, opts ...AsyncReadWrapperOption) *AsyncReadWrapper {
	wrapper := &AsyncReadWrapper{
		reader:          r,
		buffer:          &bytes.Buffer{},
		ready:           NewPollable(nil),
		done:            make(chan struct{}),
		closeUnderlying: true, // 默认为 true
	}
	for _, opt := range opts {
		opt(wrapper)
	}
	go wrapper.run()
	return wrapper
}

// run 在一个专用的后台 goroutine 中运行，它管理着另一个专门用于阻塞读取的 goroutine。
// 这是处理潜在永久阻塞 I/O 的标准 Go 模式。
func (arw *AsyncReadWrapper) run() {
	type readResult struct {
		buf []byte
		n   int
		err error
	}
	results := make(chan readResult, 1)

	// Goroutine #1: 专门负责执行阻塞的 Read 调用。
	go func() {
		defer close(results)
		for {
			buf := make([]byte, 4096)
			n, err := arw.reader.Read(buf)
			select {
			case results <- readResult{buf: buf, n: n, err: err}:
				if err != nil {
					return // 发生错误，终止读取 goroutine
				}
			case <-arw.done:
				return // 封装器被关闭，终止
			}
		}
	}()

	// Goroutine #0 (当前 run goroutine): 循环处理来自读取 goroutine 的结果。
	for {
		select {
		case res, ok := <-results:
			if !ok {
				// 读取 channel 已关闭，说明读取 goroutine 已退出。
				return
			}

			arw.mutex.Lock()
			if res.err != nil {
				arw.err = res.err
				arw.ready.SetReady() // 发生错误，唤醒任何等待者
				arw.mutex.Unlock()
				return
			}
			if res.n > 0 {
				wasEmpty := arw.buffer.Len() == 0
				arw.buffer.Write(res.buf[:res.n])
				if wasEmpty {
					arw.ready.SetReady() // 缓冲区从空变为非空，通知等待者
				}
			}
			arw.mutex.Unlock()

		case <-arw.done:
			return // 封装器被关闭，终止
		}
	}
}

// Read 从内部缓冲区非阻塞地读取数据。
func (arw *AsyncReadWrapper) Read(p []byte) (n int, err error) {
	arw.mutex.Lock()
	defer arw.mutex.Unlock()

	if arw.buffer.Len() > 0 {
		n, _ = arw.buffer.Read(p)
		if arw.buffer.Len() == 0 && arw.err == nil {
			arw.ready.Reset() // 缓冲区已空且没有最终错误，重置 pollable
		}
		return n, nil
	}

	// 如果缓冲区为空，但后台已报告错误（如 EOF），则返回该错误。
	return 0, arw.err
}

// subscribe 返回一个 pollable，它在有数据可读或发生错误时变为就绪。
func (arw *AsyncReadWrapper) subscribe() IPollable {
	arw.mutex.Lock()
	defer arw.mutex.Unlock()
	// 立即检查当前状态，如果满足条件则立即设为就绪
	if arw.buffer.Len() > 0 || arw.err != nil {
		arw.ready.SetReady()
	}
	return arw.ready
}

// Close 停止后台 goroutine 并根据配置决定是否关闭底层资源。
func (arw *AsyncReadWrapper) Close() error {
	arw.once.Do(func() {
		close(arw.done)
	})
	// 检查标志位，决定是否关闭底层 reader
	if arw.closeUnderlying {
		if c, ok := arw.reader.(io.Closer); ok {
			return c.Close()
		}
	}
	return nil
}

// NewAsyncStreamForReader 是一个便捷辅助函数，
// 将一个阻塞的 io.Reader 转换为完全支持异步 subscribe 的 *Stream。
func NewAsyncStreamForReader(r io.Reader, opts ...AsyncReadWrapperOption) *Stream {
	wrapper := NewAsyncReadWrapper(r, opts...)
	return &Stream{
		Reader: wrapper,
		Closer: wrapper,
		OnSubscribe: func() IPollable {
			return wrapper.subscribe()
		},
	}
}

// --- Asynchronous Writer ---

// AsyncWriteWrapperOption 是用于配置 AsyncWriteWrapper 的函数类型。
type AsyncWriteWrapperOption func(*AsyncWriteWrapper)

// DontCloseWriter 是一个选项，用于阻止在关闭 wrapper 时关闭底层的 writer。
func DontCloseWriter() AsyncWriteWrapperOption {
	return func(aww *AsyncWriteWrapper) {
		aww.closeUnderlying = false
	}
}

func WriterWritten(bytesWritten *atomic.Uint64) AsyncWriteWrapperOption {
	return func(aww *AsyncWriteWrapper) {
		aww.bytesWritten = bytesWritten
	}
}

func WithMaxBufferSize(size int) AsyncWriteWrapperOption {
	return func(aww *AsyncWriteWrapper) {
		aww.maxBufferSize = size
	}
}

// AsyncWriteWrapper 将一个阻塞的 io.Writer 封装成一个非阻塞的 writer，
// 带有内部缓冲区，并通过 IPollable 接口提供空间可用性通知。
type AsyncWriteWrapper struct {
	writer          io.Writer
	buffer          *bytes.Buffer
	mutex           sync.Mutex
	cond            *sync.Cond
	ready           *ChannelPollable
	done            chan struct{}
	err             error
	maxBufferSize   int
	once            sync.Once
	closeUnderlying bool
	bytesWritten    *atomic.Uint64
}

// NewAsyncWriteWrapper 创建并启动一个新的异步写入封装器。
func NewAsyncWriteWrapper(w io.Writer, opts ...AsyncWriteWrapperOption) *AsyncWriteWrapper {
	wrapper := &AsyncWriteWrapper{
		writer:          w,
		buffer:          &bytes.Buffer{},
		ready:           NewPollable(nil),
		done:            make(chan struct{}),
		maxBufferSize:   defaultBufferSize,
		closeUnderlying: true,
	}
	for _, opt := range opts {
		opt(wrapper)
	}
	wrapper.cond = sync.NewCond(&wrapper.mutex)
	wrapper.ready.SetReady() // 一开始缓冲区是空的，所以是就绪状态
	go wrapper.run()
	return wrapper
}

func (aww *AsyncWriteWrapper) run() {
	for {
		aww.mutex.Lock()
		for aww.buffer.Len() == 0 {
			select {
			case <-aww.done:
				aww.mutex.Unlock()
				return
			default:
				aww.cond.Wait()
			}
		}

		tempBuf := make([]byte, aww.buffer.Len())
		_, _ = aww.buffer.Read(tempBuf)
		aww.mutex.Unlock()

		n, err := aww.writer.Write(tempBuf)

		aww.mutex.Lock()
		if n > 0 && aww.bytesWritten != nil {
			aww.bytesWritten.Add(uint64(n))
		}
		if err != nil {
			aww.err = err
		}
		aww.ready.SetReady()
		aww.cond.Broadcast() // 唤醒可能在等待 Flush 的 goroutine
		aww.mutex.Unlock()
	}
}

func (aww *AsyncWriteWrapper) Write(p []byte) (n int, err error) {
	aww.mutex.Lock()
	defer aww.mutex.Unlock()

	if aww.err != nil {
		return 0, aww.err
	}
	available := aww.maxBufferSize - aww.buffer.Len()
	if available <= 0 {
		return 0, nil
	}
	if len(p) > available {
		n, _ = aww.buffer.Write(p[:available])
	} else {
		n, _ = aww.buffer.Write(p)
	}
	if n > 0 {
		aww.cond.Signal()
	}
	if aww.buffer.Len() >= aww.maxBufferSize {
		aww.ready.Reset()
	}
	return n, nil
}

// 新增: BlockingFlush 会阻塞直到内部缓冲区被完全写入底层 writer。
func (aww *AsyncWriteWrapper) BlockingFlush() error {
	aww.mutex.Lock()
	defer aww.mutex.Unlock()

	// 循环直到缓冲区为空或发生错误
	for aww.buffer.Len() > 0 && aww.err == nil {
		aww.cond.Signal() // 确保后台 goroutine 正在工作
		aww.cond.Wait()   // 等待后台 goroutine 完成写入并发出信号
	}
	return aww.err
}

// 新增: Flush 触发一次非阻塞的刷新。
func (aww *AsyncWriteWrapper) Flush() error {
	aww.mutex.Lock()
	defer aww.mutex.Unlock()
	aww.cond.Signal() // 唤醒后台 goroutine
	return nil
}

func (aww *AsyncWriteWrapper) CheckWrite() uint64 {
	aww.mutex.Lock()
	defer aww.mutex.Unlock()
	return uint64(aww.maxBufferSize - aww.buffer.Len())
}

func (aww *AsyncWriteWrapper) subscribe() IPollable {
	aww.mutex.Lock()
	defer aww.mutex.Unlock()
	if aww.buffer.Len() < aww.maxBufferSize || aww.err != nil {
		aww.ready.SetReady()
	}
	return aww.ready
}

func (aww *AsyncWriteWrapper) Close() error {
	// 先刷空所有数据
	if err := aww.BlockingFlush(); err != nil {
		// 即使 flush 失败，也要继续关闭
		_ = aww.closeInternal()
		return err
	}
	return aww.closeInternal()
}

func (aww *AsyncWriteWrapper) closeInternal() error {
	aww.once.Do(func() {
		aww.mutex.Lock()
		close(aww.done)
		aww.cond.Broadcast()
		aww.mutex.Unlock()
	})

	if aww.closeUnderlying {
		if c, ok := aww.writer.(io.Closer); ok {
			return c.Close()
		}
	}
	return nil
}

// NewAsyncStreamForWriter 是一个便捷辅助函数，
// 将一个阻塞的 io.Writer 转换为完全支持异步 subscribe 的 *Stream。
func NewAsyncStreamForWriter(w io.Writer, opts ...AsyncWriteWrapperOption) *Stream {
	wrapper := NewAsyncWriteWrapper(w, opts...)
	return &Stream{
		Writer:      wrapper,
		Closer:      wrapper,
		Flusher:     wrapper,
		CheckWriter: wrapper,
		OnSubscribe: func() IPollable {
			return wrapper.subscribe()
		},
	}
}
