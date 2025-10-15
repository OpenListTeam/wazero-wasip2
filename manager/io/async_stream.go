package io

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
)

// 默认的读取缓冲区大小，用于后台读取操作。
const defaultBufferSize = 8192

// --- 优化后的异步读取封装器 ---

// AsyncReadWrapperOption 是用于配置 AsyncReadWrapper 的函数类型。
type AsyncReadWrapperOption func(*AsyncReadWrapper)

// DontCloseReader 是一个选项，用于阻止在关闭 wrapper 时关闭底层的 reader。
func DontCloseReader() AsyncReadWrapperOption {
	return func(arw *AsyncReadWrapper) {
		arw.closeUnderlying = false
	}
}

// AsyncReadWrapper 将一个阻塞的 io.Reader 封装成一个非阻塞的 reader。
// 它在后台持续地从底层 reader 读取数据，并将其存入内部缓冲区。
type AsyncReadWrapper struct {
	reader          io.Reader
	buffer          *bytes.Buffer
	mutex           sync.Mutex
	ready           *ChannelPollable
	done            chan struct{}
	err             error
	once            sync.Once
	closeUnderlying bool
}

// NewAsyncReadWrapper 创建并启动一个新的异步读取封装器。
func NewAsyncReadWrapper(r io.Reader, opts ...AsyncReadWrapperOption) *AsyncReadWrapper {
	wrapper := &AsyncReadWrapper{
		reader:          r,
		buffer:          &bytes.Buffer{},
		ready:           NewPollable(nil),
		done:            make(chan struct{}),
		closeUnderlying: true, // 默认在 Close 时关闭底层 reader。
	}
	for _, opt := range opts {
		opt(wrapper)
	}
	// 启动后台读取 goroutine。
	go wrapper.run()
	return wrapper
}

// run 是在后台运行的 goroutine，它持续地执行阻塞读取操作并填充缓冲区。
func (arw *AsyncReadWrapper) run() {
	defer func() {
		// 确保在 goroutine 退出时，任何等待的消费者都能被唤醒。
		arw.mutex.Lock()
		arw.ready.SetReady()
		arw.mutex.Unlock()
	}()

	readBuf := make([]byte, defaultBufferSize)
	for {
		// 检查是否有关闭信号。
		select {
		case <-arw.done:
			return
		default:
		}

		// 执行阻塞读取。当没有数据时，goroutine 会在这里自然地暂停。
		n, readErr := arw.reader.Read(readBuf)

		arw.mutex.Lock()
		wasEmpty := arw.buffer.Len() == 0
		if n > 0 {
			// 将读取到的数据写入内部缓冲区。
			arw.buffer.Write(readBuf[:n])
		}

		// 如果发生了错误（例如 io.EOF），记录它并准备终止 goroutine。
		if readErr != nil {
			if arw.err == nil {
				arw.err = readErr
			}
		}

		// 如果缓冲区从空变为非空，或者发生了错误，
		// 就将 pollable 设置为就绪状态，以唤醒等待的消费者。
		if (wasEmpty && arw.buffer.Len() > 0) || (readErr != nil) {
			arw.ready.SetReady()
		}
		arw.mutex.Unlock()

		// 如果遇到任何错误，就停止读取。
		if readErr != nil {
			return
		}
	}
}

// Read 从内部缓冲区非阻塞地读取数据。
// 如果缓冲区为空，它会返回 (0, nil)。
// 调用者应该使用 subscribe() 来等待数据变为可用。
func (arw *AsyncReadWrapper) Read(p []byte) (n int, err error) {
	arw.mutex.Lock()
	defer arw.mutex.Unlock()

	// 1. 检查缓冲区是否有数据。
	if arw.buffer.Len() > 0 {
		n, _ = arw.buffer.Read(p)

		// 如果这次读取耗尽了缓冲区，并且没有持久性错误（如EOF），
		// 重置 pollable 的状态，以防止虚假唤醒。
		if arw.buffer.Len() == 0 && arw.err == nil {
			arw.ready.Reset()
		}

		return n, nil
	}

	// 2. 缓冲区为空，检查是否有已记录的错误（如 EOF）。
	if arw.err != nil {
		return 0, arw.err
	}

	// 3. 缓冲区为空且无错误，表示需要等待后台 goroutine 读取更多数据。
	return 0, nil
}

// subscribe 返回一个 pollable 对象，当有数据可读或发生错误时，该对象会变为就绪状态。
func (arw *AsyncReadWrapper) subscribe() IPollable {
	arw.mutex.Lock()
	defer arw.mutex.Unlock()

	// 如果数据已在缓冲区中或已发生错误，则立即返回一个就绪的 pollable。
	if arw.buffer.Len() > 0 || arw.err != nil {
		return ReadyPollable
	}

	// 否则，重置 pollable 状态并返回，等待 run goroutine 将其设置为就绪。
	arw.ready.Reset()
	return arw.ready
}

// Close 停止后台 goroutine 并根据配置决定是否关闭底层资源。
func (arw *AsyncReadWrapper) Close() error {
	var closeErr error
	arw.once.Do(func() {
		close(arw.done)

		// 根据配置决定是否关闭底层 reader
		if arw.closeUnderlying {
			if c, ok := arw.reader.(io.Closer); ok {
				closeErr = c.Close()
			}
		}
	})
	return closeErr
}

// NewAsyncStreamForReader 是一个便捷的辅助函数，
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

// 记录写入数量
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
	return aww.BlockingFlush()
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
