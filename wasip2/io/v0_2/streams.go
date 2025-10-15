package v0_2

import (
	"context"
	"io"
	"time"

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
	s, ok := i.sm.Get(this)
	if !ok || s.Reader == nil {
		return witgo.Err[[]byte, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	if s.OnSubscribe != nil {
		if pollable := s.OnSubscribe(); pollable != nil {
			pollable.Block()
		}
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
	buf := make([]byte, 32*1024)

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
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
	}
	if s.CheckWriter != nil {
		return witgo.Ok[uint64, StreamError](s.CheckWriter.CheckWrite())
	}
	return witgo.Ok[uint64, StreamError](4096)
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
	s, ok := i.sm.Get(this)
	if !ok || s.Writer == nil {
		return witgo.Err[witgo.Unit, StreamError](StreamError{Closed: &witgo.Unit{}})
	}

	writeSize := uint64(4096)
	for len(contents) > 0 {
		if s.CheckWriter != nil {
			writeSize = s.CheckWriter.CheckWrite()
		}
		// 如果没有可写入空间，那么阻塞等待IO就绪
		if writeSize == 0 {
			if s.OnSubscribe != nil {
				if pollable := s.OnSubscribe(); pollable != nil {
					pollable.Block()
					continue
				}
			}
			time.Sleep(time.Millisecond * 20)
		}

		n, err := s.Writer.Write(contents[:min(writeSize, uint64(len(contents)))])
		if err != nil {
			errHandle := i.em.Add(err)
			return witgo.Err[witgo.Unit, StreamError](StreamError{LastOperationFailed: &errHandle})
		}
		contents = contents[n:]
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
	return i.subscribeToStream(this)
}

// Splice 现在是所有阻塞式数据传输的主要实现。
// 它会一直等待，直到 maxLen 字节被完整传输或发生错误。
func (i *streamsImpl) Splice(ctx context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	dst, ok := i.sm.Get(this)
	if !ok || dst.Writer == nil {
		return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
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
			return witgo.Err[uint64, StreamError](StreamError{Closed: &witgo.Unit{}})
		}
		srcReader = srcStream.Reader
	}

	var totalWritten uint64
	// 循环直到所有 maxLen 字节都被成功写入。
	for totalWritten < maxLen {
		// 1. 检查目标流中可用的写入空间。
		writePermit := uint64(4096)
		if dst.CheckWriter != nil {
			writePermit = dst.CheckWriter.CheckWrite()
		}

		// 2. 如果没有可用空间，我们必须阻塞等待。
		if writePermit == 0 {
			// 这个逻辑与阻塞读取的逻辑相似。我们等待流再次变为可写状态。
			if dst.OnSubscribe != nil {
				if pollable := dst.OnSubscribe(); pollable != nil {
					pollable.Block()
					continue // 重启循环以再次检查可用空间。
				}
			}
			// 如果没有订阅机制，则使用固定的 sleep 作为备用方案。
			time.Sleep(20 * time.Millisecond)
			continue
		}

		// 3. 决定下一个要写入的数据块大小。
		remaining := maxLen - totalWritten
		chunkSize := min(writePermit, remaining)

		// 4. 将数据块从源复制到目标。
		n, err := io.CopyN(dst.Writer, srcReader, int64(chunkSize))
		totalWritten += uint64(n)

		if err != nil && err != io.EOF {
			errHandle := i.em.Add(err)
			return witgo.Err[uint64, StreamError](StreamError{LastOperationFailed: &errHandle})
		}

		// 如果在写完 maxLen 之前，源流就结束了 (EOF)，这也是正常情况。
		// 我们已经拼接了所有能拼接的数据，因此跳出循环。
		if err == io.EOF {
			break
		}
	}
	return witgo.Ok[uint64, StreamError](totalWritten)
}

// BlockingSplice 现在与 Splice 的行为保持一致，因为所有写入操作都已是阻塞的。
func (i *streamsImpl) BlockingSplice(ctx context.Context, this OutputStream, src InputStream, maxLen uint64) witgo.Result[uint64, StreamError] {
	return i.Splice(ctx, this, src, maxLen)
}

// WriteZeroes 从 Splice 继承了新的阻塞行为。
func (i *streamsImpl) WriteZeroes(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	// 调用阻塞式的 Splice，并使用一个虚拟的零字节流 (src=0) 作为源。
	if spliceResult := i.Splice(ctx, this, 0, len); spliceResult.Err != nil {
		return witgo.Err[witgo.Unit, StreamError](*spliceResult.Err)
	}
	return witgo.Ok[witgo.Unit, StreamError](witgo.Unit{})
}

// BlockingWriteZeroesAndFlush
func (i *streamsImpl) BlockingWriteZeroesAndFlush(ctx context.Context, this OutputStream, len uint64) witgo.Result[witgo.Unit, StreamError] {
	if writeResult := i.WriteZeroes(ctx, this, len); writeResult.Err != nil {
		return writeResult
	}
	return i.BlockingFlush(ctx, this)
}

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
