package v0_2

import (
	"context"
	"io"
	"math"
	"wazero-wasip2/internal/errors"
	"wazero-wasip2/internal/poll"
	"wazero-wasip2/internal/streams"
	witgo "wazero-wasip2/wit-go"
)

// streamsImpl 结构体持有 wasi:io/streams 的具体实现逻辑。
// 它通过依赖注入获得了所有需要的状态管理器。
type streamsImpl struct {
	sm *streams.Manager
	em *errors.ResourceManager
	pm *poll.Manager
}

// newStreamsImpl 创建一个新的 streamsImpl 实例。
func newStreamsImpl(sm *streams.Manager, em *errors.ResourceManager, pm *poll.Manager) *streamsImpl {
	return &streamsImpl{sm: sm, em: em, pm: pm}
}

// --- 资源管理 ---

func (i *streamsImpl) DropInputStream(_ context.Context, handle InputStream) {
	// 如果流有关联的 Closer，先关闭它。
	if s, ok := i.sm.Get(handle); ok && s.Closer != nil {
		s.Closer.Close()
	}
	i.sm.Remove(handle)
}

func (i *streamsImpl) DropOutputStream(_ context.Context, handle OutputStream) {
	if s, ok := i.sm.Get(handle); ok && s.Closer != nil {
		s.Closer.Close()
	}
	i.sm.Remove(handle)
}

// --- input-stream 方法实现 ---
func (i *streamsImpl) Read(_ context.Context, this InputStream, maxLen uint64) witgo.Result[[]byte, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[[]byte, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	buf := make([]byte, maxLen)
	n, err := s.Reader.Read(buf)

	if n == 0 && err != nil {
		if err == io.EOF {
			return witgo.Err[[]byte, StreamError](StreamError{Closed: &witgo.Unit{}})
		}
		errHandle := i.em.Add(err)
		return witgo.Err[[]byte, StreamError](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[[]byte, StreamError](buf[:n])
}

func (i *streamsImpl) BlockingRead(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[[]byte, StreamError] {
	// 完整的实现需要 pollable，但为简化，我们先复用非阻塞版本。
	// 一个真实的实现会先 block() 在 subscribe() 返回的 pollable 上。
	return i.Read(ctx, this, maxLen)
}

func (i *streamsImpl) Skip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	// 这是 BlockingSkip 的逻辑，但我们复用于 Skip
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}
	n, err := io.CopyN(io.Discard, s.Reader, int64(maxLen))
	if err != nil && err != io.EOF {
		errHandle := i.em.Add(err)
		return witgo.Err[uint64, StreamError](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[uint64, StreamError](uint64(n))
}

func (i *streamsImpl) BlockingSkip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	return i.Skip(ctx, this, maxLen)
}

func (i *streamsImpl) SubscribeToInputStream(_ context.Context, this InputStream) Pollable {
	_, ok := i.sm.Get(this)
	ch := make(chan struct{})
	if !ok {
		// 无效的流，返回一个立即就绪的 pollable
		close(ch)
	} else {
		// 这是一个简化的实现。一个完整的实现需要一个 goroutine 在后台
		// 监测底层 I/O 对象的可读状态，并在就绪时 close(ch)。
		// 为了演示，我们假设流总是可以立即尝试读取的。
		close(ch)
	}
	return i.pm.Add(ch)
}

// --- output-stream 方法实现 ---

func (i *streamsImpl) CheckWrite(_ context.Context, this OutputStream) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}
	// 返回一个较大的值，表示通常可以写入。
	return witgo.Ok[uint64, StreamError](math.MaxUint64)
}

func (i *streamsImpl) Write(_ context.Context, this OutputStream, contents []byte) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit, StreamError](StreamError{Closed: &witgo.Unit{}})
	}
	_, err := s.Writer.Write(contents)
	if err != nil {
		errHandle := i.em.Add(err)
		return witgo.Err[witgo.Unit, StreamError](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingWriteAndFlush(ctx context.Context, this OutputStream, contents []byte) witgo.Result[witgo.Unit, StreamError] {
	writeResult := i.Write(ctx, this, contents)
	if writeResult.Err != nil {
		return writeResult
	}
	return i.BlockingFlush(ctx, this)
}

func (i *streamsImpl) Flush(_ context.Context, this OutputStream) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	// 关键改动：检查 stream 是否实现了 Flusher 接口。
	if s.Flusher != nil {
		if err := s.Flusher.Flush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit, StreamError](StreamError{LastOperationFailed: &errHandle})
		}
	}

	// 如果没有 Flusher，此操作为空操作，并成功返回。
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingFlush(ctx context.Context, this OutputStream) witgo.Result[witgo.Unit, StreamError] {
	return i.Flush(ctx, this)
}

func (i *streamsImpl) SubscribeToOutputStream(_ context.Context, this OutputStream) Pollable {
	_, ok := i.sm.Get(this)
	ch := make(chan struct{})
	if !ok {
		close(ch)
	} else {
		// 简化实现，假设流总是可写的。
		close(ch)
	}
	return i.pm.Add(ch)
}

func (i *streamsImpl) WriteZeroes(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	// 我们可以用 Splice 来巧妙地实现
	result := i.Splice(ctx, this, 0, len)
	if result.Err != nil {
		return witgo.Result[witgo.Unit, StreamError]{
			Err: result.Err,
		}
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingWriteZeroesAndFlush(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	writeResult := i.WriteZeroes(ctx, this, len)
	if writeResult.Err != nil {
		return writeResult
	}
	return i.BlockingFlush(ctx, this)
}

func (i *streamsImpl) Splice(_ context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	dst, ok := i.sm.Get(this)
	if !ok || dst.Writer == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	var srcReader io.Reader
	// 特殊处理 src = 0 的情况，这在 write-zeroes 中很有用
	if src == 0 {
		srcReader = &io.LimitedReader{R: zeroReader{}, N: int64(maxLen)}
	} else {
		srcStream, ok := i.sm.Get(src)
		if !ok || srcStream.Reader == nil {
			return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
		}
		srcReader = srcStream.Reader
	}

	n, err := io.CopyN(dst.Writer, srcReader, int64(maxLen))
	if err != nil && err != io.EOF {
		errHandle := i.em.Add(err)
		return witgo.Err[uint64, StreamError](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[uint64, StreamError](uint64(n))
}

func (i *streamsImpl) BlockingSplice(ctx context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	return i.Splice(ctx, this, src, maxLen)
}

// zeroReader 是一个 io.Reader，它总是读取零字节。
type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
