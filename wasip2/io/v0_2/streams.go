package v0_2

import (
	"context"
	"io"
	"time"

	"github.com/OpenListTeam/wazero-wasip2/common/bytespool"
	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
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
func (i *streamsImpl) subscribeToStream(this uint32) Pollable {
	s, ok := i.sm.Get(this)
	if !ok {
		// 无效的 stream 句柄，返回一个立即就绪的 pollable。
		return i.pm.Add(manager_io.ReadyPollable)
	}

	// 如果 stream 的创建者提供了 OnSubscribe 回调，则调用它。
	if s.OnSubscribe != nil {
		pollable := s.OnSubscribe()
		if pollable != nil {
			return i.pm.Add(pollable)
		}
	}

	// 否则，回退到默认行为：为通用阻塞流创建一个立即就绪的 pollable。
	return i.pm.Add(manager_io.ReadyPollable)
}

func (i *streamsImpl) DropInputStream(_ context.Context, handle InputStream) {
	if s, ok := i.sm.Pop(handle); ok && s.Closer != nil {
		s.Closer.Close()
	}
}

func (i *streamsImpl) DropOutputStream(_ context.Context, handle OutputStream) {
	if s, ok := i.sm.Pop(handle); ok && s.Closer != nil {
		s.Closer.Close()
	}
}

func (i *streamsImpl) Read(_ context.Context, this InputStream, maxLen uint64) witgo.Result[[]byte, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[[]byte](StreamError{Closed: &witgo.Unit{}})
	}

	buf := make([]byte, maxLen)
	n, err := s.Reader.Read(buf)

	if n == 0 && err != nil {
		if err == io.EOF {
			return witgo.Err[[]byte](StreamError{Closed: &witgo.Unit{}})
		}
		errHandle := i.em.Add(err)
		return witgo.Err[[]byte](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[[]byte, StreamError](buf[:n])
}

func (i *streamsImpl) BlockingRead(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[[]byte, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[[]byte](StreamError{Closed: &witgo.Unit{}})
	}
	buf := make([]byte, maxLen)
	n, err := io.ReadAtLeast(newBlockingReader(ctx, s), buf, 1)
	if n == 0 && err != nil {
		if err == io.EOF {
			return witgo.Err[[]byte](StreamError{Closed: &witgo.Unit{}})
		}
		errHandle := i.em.Add(err)
		return witgo.Err[[]byte](StreamError{LastOperationFailed: &errHandle})
	}
	return witgo.Ok[[]byte, StreamError](buf[:n])
}

// Skip (非阻塞) 尝试跳过最多 maxLen 字节并立即返回。
func (i *streamsImpl) Skip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	// 优先使用 Seeker 实现高效跳转。
	if s.Seeker != nil {
		currentPos, err := s.Seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			newPos, err := s.Seeker.Seek(int64(maxLen), io.SeekCurrent)
			if err == nil {
				return witgo.Ok[uint64, StreamError](uint64(newPos - currentPos))
			}
		}
	}

	// 回退到读取和丢弃方法，并指定为非阻塞模式。
	return i.skipByReading(s, maxLen, false) // blocking = false
}

// BlockingSkip (阻塞) 会跳过 maxLen 字节，并在必要时等待数据。
func (i *streamsImpl) BlockingSkip(ctx context.Context, this InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	// 优先使用 Seeker 实现高效跳转。
	if s.Seeker != nil {
		currentPos, err := s.Seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			newPos, err := s.Seeker.Seek(int64(maxLen), io.SeekCurrent)
			if err == nil {
				return witgo.Ok[uint64, StreamError](uint64(newPos - currentPos))
			}
		}
	}

	// 回退到读取和丢弃方法，并指定为阻塞模式。
	return i.skipByReading(s, maxLen, true) // blocking = true
}

// skipByReading 是跳过字节的核心实现，支持阻塞和非阻塞两种模式。
func (i *streamsImpl) skipByReading(s *manager_io.Stream, maxLen uint64, blocking bool) witgo.Result[uint64, StreamError] {
	var totalSkipped uint64
	buf := bytespool.Alloc(32 * 1024)
	defer bytespool.Free(buf)

	for totalSkipped < maxLen {
		readSize := uint64(len(buf))
		if remaining := maxLen - totalSkipped; remaining < readSize {
			readSize = remaining
		}

		n, err := s.Reader.Read(buf[:readSize])
		if n > 0 {
			totalSkipped += uint64(n)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			errHandle := i.em.Add(err)
			return witgo.Err[uint64, StreamError](StreamError{LastOperationFailed: &errHandle})
		}

		// 当 n == 0 时，根据 blocking 参数决定行为
		if n == 0 {
			if blocking {
				// 阻塞模式：等待更多数据
				if s.OnSubscribe != nil {
					if pollable := s.OnSubscribe(); pollable != nil {
						pollable.Block()
						continue // 继续循环以尝试再次读取
					}
				}
				time.Sleep(time.Millisecond * 20)
			} else {
				// 非阻塞模式：立即停止
				break
			}
		}
	}

	return witgo.Ok[uint64, StreamError](totalSkipped)
}

func (i *streamsImpl) SubscribeToInputStream(_ context.Context, this InputStream) Pollable {
	return i.subscribeToStream(this)
}

func (i *streamsImpl) CheckWrite(_ context.Context, this OutputStream) witgo.Result[uint64, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[uint64](StreamError{Closed: &witgo.Unit{}})
	}
	if s.CheckWriter != nil {
		return witgo.Ok[uint64, StreamError](s.CheckWriter.CheckWrite())
	}
	return witgo.Ok[uint64, StreamError](4096)
}

func (i *streamsImpl) Write(_ context.Context, this OutputStream, contents []byte) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit](StreamError{Closed: &witgo.Unit{}})
	}
	// Write要配合CheckWrite使用的，所以直接写入
	for len(contents) > 0 {
		n, err := s.Writer.Write(contents)
		if err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit](StreamError{LastOperationFailed: &errHandle})
		}
		contents = contents[n:]
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingWriteAndFlush(ctx context.Context, this OutputStream, contents []byte) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit](StreamError{Closed: &witgo.Unit{}})
	}
	writeSize := uint64(len(contents))
	if writeSize > 4096 {
		panic("WASI: blocking-write-and-flush called with contents length > 4096. Guest must handle chunking.")
	}

	dstWriter := newBlockingWriter(ctx, s)
	for len(contents) > 0 {
		n, err := dstWriter.Write(contents)
		if err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit](StreamError{LastOperationFailed: &errHandle})
		}
		contents = contents[n:]
	}

	if s.Flusher != nil {
		if err := s.Flusher.BlockingFlush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit](StreamError{LastOperationFailed: &errHandle})
		}
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) Flush(_ context.Context, this OutputStream) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit](StreamError{Closed: &witgo.Unit{}})
	}

	if s.Flusher != nil {
		if err := s.Flusher.Flush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit](StreamError{LastOperationFailed: &errHandle})
		}
	}

	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) BlockingFlush(ctx context.Context, this OutputStream) witgo.Result[witgo.Unit, StreamError] {
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit](StreamError{Closed: &witgo.Unit{}})
	}

	if s.Flusher != nil {
		if err := s.Flusher.BlockingFlush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit](StreamError{LastOperationFailed: &errHandle})
		}
	}

	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

func (i *streamsImpl) SubscribeToOutputStream(_ context.Context, this OutputStream) Pollable {
	return i.subscribeToStream(this)
}

func (i *streamsImpl) Splice(ctx context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	dst, ok := i.sm.Get(this)
	if !ok || dst.Writer == nil {
		return witgo.Err[uint64](StreamError{Closed: &witgo.Unit{}})
	}

	// 设置源读取器。如果 src 为 0，我们使用一个特殊的 zeroReader。
	var srcReader io.Reader
	if src == 0 {
		// 对于 WriteZeroes，数据源是一个无穷的零字节流。
		// 此处我们不需要 LimitedReader，因为外层循环已经控制了总长度。
		srcReader = zeroReader{}
	} else {
		srcStream, ok := i.sm.Get(src)
		if !ok || srcStream.Reader == nil {
			return witgo.Err[uint64](StreamError{Closed: &witgo.Unit{}})
		}
		srcReader = srcStream.Reader
	}
	buf := bytespool.Alloc(32 * 1024)
	defer bytespool.Free(buf)
	writeSize := maxLen
F:
	for writeSize > 0 {
		readSize := len(buf)
		if dst.CheckWriter != nil {
			readSize = min(readSize, int(dst.CheckWriter.CheckWrite()))
		}
		readSize, err := srcReader.Read(buf[:readSize])
		if readSize > 0 {
			data := buf[:readSize]
			for len(data) > 0 {
				written, writeErr := dst.Writer.Write(data)
				writeSize -= uint64(written)
				if writeErr != nil {
					err = writeErr
					break
				}
				data = data[written:]
			}
		}
		switch err {
		case nil:
			if readSize == 0 {
				break F
			}
			// 正常读写，继续处理。
		case io.EOF:
			// 源流已结束，退出循环。
			break F
		default:
			// 其他错误，记录并返回。
			errHandle := i.em.Add(err)
			return witgo.Err[uint64](StreamError{LastOperationFailed: &errHandle})
		}
	}

	return witgo.Ok[uint64, StreamError](maxLen - uint64(writeSize))
}

type blockingStream struct {
	ctx context.Context
	*manager_io.Stream
}

func (bs *blockingStream) Read(p []byte) (n int, err error) {
	if pollable := bs.Stream.OnSubscribe(); pollable != nil {
		select {
		case <-bs.ctx.Done():
			return 0, bs.ctx.Err()
		case <-pollable.Channel():
		}
	}
	n, err = bs.Stream.Reader.Read(p)
	return
}

func newBlockingReader(ctx context.Context, s *manager_io.Stream) io.Reader {
	if s.OnSubscribe != nil {
		return &blockingStream{
			ctx:    ctx,
			Stream: s,
		}
	}
	return s.Reader
}

func (bs *blockingStream) Write(p []byte) (n int, err error) {
	writeSize := uint64(len(p))
	for {
		checkSize := bs.Stream.CheckWriter.CheckWrite()
		if checkSize > 0 {
			if checkSize < writeSize {
				writeSize = checkSize
			}
			break
		}
		if bs.Stream.OnSubscribe != nil {
			if pollable := bs.Stream.OnSubscribe(); pollable != nil {
				select {
				case <-bs.ctx.Done():
					return 0, bs.ctx.Err()
				case <-pollable.Channel():
				}
			}
			continue
		}
		select {
		case <-bs.ctx.Done():
			return 0, bs.ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}

	n, err = bs.Stream.Writer.Write(p[:writeSize])
	return
}
func newBlockingWriter(ctx context.Context, s *manager_io.Stream) io.Writer {
	if s.CheckWriter != nil {
		return &blockingStream{
			ctx:    ctx,
			Stream: s,
		}
	}
	return s.Writer
}

// BlockingSplice 现在与 Splice 的行为保持一致，因为所有写入操作都已是阻塞的。
func (i *streamsImpl) BlockingSplice(ctx context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	dst, ok := i.sm.Get(this)
	if !ok || dst.Writer == nil {
		return witgo.Err[uint64](StreamError{Closed: &witgo.Unit{}})
	}

	// 设置源读取器。如果 src 为 0，我们使用一个特殊的 zeroReader。
	var srcReader io.Reader
	if src == 0 {
		// 对于 WriteZeroes，数据源是一个无穷的零字节流。
		// 此处我们不需要 LimitedReader，因为外层循环已经控制了总长度。
		srcReader = zeroReader{}
	} else {
		srcStream, ok := i.sm.Get(src)
		if !ok || srcStream.Reader == nil {
			return witgo.Err[uint64](StreamError{Closed: &witgo.Unit{}})
		}
		srcReader = newBlockingReader(ctx, srcStream)
	}

	buf := bytespool.Alloc(32 * 1024)
	defer bytespool.Free(buf)
	n, err := io.CopyBuffer(newBlockingWriter(ctx, dst), io.LimitReader(srcReader, int64(maxLen)), buf)
	switch err {
	case nil:
		// 成功写入所有请求的字节，继续循环直到完成。
	case io.EOF:
		// 源流已结束，退出循环。
	default:
		// 其他错误，记录并返回。
		errHandle := i.em.Add(err)
		return witgo.Err[uint64](StreamError{LastOperationFailed: &errHandle})
	}

	if dst.Flusher != nil {
		if err := dst.Flusher.BlockingFlush(); err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[uint64](StreamError{LastOperationFailed: &errHandle})
		}
	}
	return witgo.Ok[uint64, StreamError](uint64(n))
}

// WriteZeroes 从 Splice 继承了新的阻塞行为。
func (i *streamsImpl) WriteZeroes(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	// 调用阻塞式的 Splice，并使用一个虚拟的零字节流 (src=0) 作为源。
	if spliceResult := i.Splice(ctx, this, 0, len); spliceResult.Err != nil {
		return witgo.Err[witgo.Unit](*spliceResult.Err)
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

// BlockingWriteZeroesAndFlush
func (i *streamsImpl) BlockingWriteZeroesAndFlush(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	if spliceResult := i.BlockingSplice(ctx, this, 0, len); spliceResult.Err != nil {
		return witgo.Err[witgo.Unit](*spliceResult.Err)
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (n int, err error) {
	clear(p)
	return len(p), nil
}
