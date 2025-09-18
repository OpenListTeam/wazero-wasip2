package v0_2

import (
	"context"
	"io"
	"math"
	manager_io "wazero-wasip2/internal/io"
	witgo "wazero-wasip2/wit-go"
)

type streamsImpl struct {
	sm *manager_io.StreamManager
	em *manager_io.ErrorManager
	pm *manager_io.PollManager
}

func newStreamsImpl(sm *manager_io.StreamManager, em *manager_io.ErrorManager, pm *manager_io.PollManager) *streamsImpl {
	return &streamsImpl{sm: sm, em: em, pm: pm}
}

// subscribeToStream 是 SubscribeToInputStream 和 SubscribeToOutputStream 的通用实现。
func (i *streamsImpl) subscribeToStream(this uint32, direction manager_io.PollDirection) Pollable {
	s, ok := i.sm.Get(this)
	if !ok {
		// 无效的 stream 句柄，返回一个立即就绪的 pollable，以便上层能尽快发现错误。
		p := manager_io.NewPollable(nil)
		close(p.ReadyChan)
		return i.pm.Add(p)
	}

	// 如果流有关联的 Fd，我们创建一个携带 Fd 和方向的 Pollable
	if s.Fd != 0 {
		p := manager_io.NewPollable(nil)
		p.Fd = s.Fd
		p.Direction = direction
		return i.pm.Add(p)
	}

	// 如果 stream 的创建者提供了 OnSubscribe 回调，则调用它。
	if s.OnSubscribe != nil {
		return s.OnSubscribe()
	}

	// 否则，回退到默认行为：为通用阻塞流创建一个立即就绪的 pollable。
	p := manager_io.NewPollable(nil)
	close(p.ReadyChan)
	return i.pm.Add(p)
}

func (i *streamsImpl) DropInputStream(_ context.Context, handle InputStream) {
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
	return i.Read(ctx, this, maxLen)
}

// Skip by reading and discarding data. This is the fallback method.
func (i *streamsImpl) skipByReading(s *manager_io.Stream, maxLen uint64) witgo.Result[uint64, StreamError] {
	n, err := io.CopyN(io.Discard, s.Reader, int64(maxLen))
	if err != nil && err != io.EOF {
		errHandle := i.em.Add(err)
		return witgo.Err[uint64, StreamError](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[uint64, StreamError](uint64(n))
}

func (i *streamsImpl) Skip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	// 优先使用 Seeker 实现高效跳转
	if s.Seeker != nil {
		// Seek with 0 offset to get current position
		currentPos, err := s.Seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			// Seek forward by maxLen
			newPos, err := s.Seeker.Seek(int64(maxLen), io.SeekCurrent)
			if err == nil {
				return witgo.Ok[uint64, StreamError](uint64(newPos - currentPos))
			}
		}
		// If seeking fails for any reason, fall through to skip by reading.
	}

	// Fallback for streams that don't implement Seeker, or if Seek fails.
	return i.skipByReading(s, maxLen)
}

func (i *streamsImpl) BlockingSkip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	return i.Skip(ctx, this, maxLen)
}

func (i *streamsImpl) SubscribeToInputStream(_ context.Context, this InputStream) Pollable {
	return i.subscribeToStream(this, manager_io.PollDirectionRead)
}

func (i *streamsImpl) CheckWrite(_ context.Context, this OutputStream) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}
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

	if s.Flusher != nil {
		if err := s.Flusher.Flush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit, StreamError](StreamError{LastOperationFailed: &errHandle})
		}
	}

	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingFlush(ctx context.Context, this OutputStream) witgo.Result[witgo.Unit, StreamError] {
	return i.Flush(ctx, this)
}

func (i *streamsImpl) SubscribeToOutputStream(_ context.Context, this OutputStream) Pollable {
	return i.subscribeToStream(this, manager_io.PollDirectionWrite)
}

func (i *streamsImpl) WriteZeroes(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
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

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
