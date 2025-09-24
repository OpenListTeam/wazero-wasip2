package v0_2

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"wazero-wasip2/internal/filesystem"
	manager_io "wazero-wasip2/internal/io"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"
)

type typesImpl struct {
	host *wasip2.Host
}

func newTypesImpl(h *wasip2.Host) *typesImpl {
	return &typesImpl{host: h}
}

/// TODO:
/// 1. 完善沙盒机制
/// 2. 完善poll机制(可能没有必要)

// --- descriptor resource methods ---

func (i *typesImpl) DropDescriptor(_ context.Context, handle Descriptor) {
	if d, ok := i.host.FilesystemManager().Get(handle); ok {
		d.File.Close()
	}
	i.host.FilesystemManager().Remove(handle)
}

func (i *typesImpl) ReadViaStream(_ context.Context, this Descriptor, offset Filesize) witgo.Result[InputStream, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[InputStream, ErrorCode](ErrorCodeBadDescriptor)
	}
	reader := io.NewSectionReader(d.File, int64(offset), -1)
	stream := &manager_io.Stream{Reader: reader, Seeker: reader}
	handle := i.host.StreamManager().Add(stream)
	return witgo.Ok[InputStream, ErrorCode](handle)
}

func (i *typesImpl) WriteViaStream(_ context.Context, this Descriptor, offset Filesize) witgo.Result[OutputStream, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[OutputStream, ErrorCode](ErrorCodeBadDescriptor)
	}
	writer := &sectionWriter{d.File, int64(offset)}
	stream := &manager_io.Stream{Writer: writer}
	handle := i.host.StreamManager().Add(stream)
	return witgo.Ok[OutputStream, ErrorCode](handle)
}

func (i *typesImpl) AppendViaStream(_ context.Context, this Descriptor) witgo.Result[OutputStream, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[OutputStream, ErrorCode](ErrorCodeBadDescriptor)
	}
	stream := &manager_io.Stream{Writer: d.File, Flusher: &OsFileFlusher{w: d.File}}
	handle := i.host.StreamManager().Add(stream)
	return witgo.Ok[OutputStream, ErrorCode](handle)
}

func (i *typesImpl) Advise(ctx context.Context, this Descriptor, offset Filesize, length Filesize, advice Advice) witgo.Result[witgo.Unit, ErrorCode] {
	return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeUnsupported)
}

func (i *typesImpl) SyncData(ctx context.Context, this Descriptor) witgo.Result[witgo.Unit, ErrorCode] {
	return i.Sync(ctx, this)
}

func (i *typesImpl) GetType(ctx context.Context, this Descriptor) witgo.Result[DescriptorType, ErrorCode] {
	statResult := i.Stat(ctx, this)
	if statResult.Err != nil {
		return witgo.Err[DescriptorType, ErrorCode](*statResult.Err)
	}
	return witgo.Ok[DescriptorType, ErrorCode](statResult.Ok.Type)
}

func (i *typesImpl) SetSize(ctx context.Context, this Descriptor, size Filesize) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	err := d.File.Truncate(int64(size))
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) SetTimes(ctx context.Context, this Descriptor, data_access_timestamp NewTimestamp, data_modification_timestamp NewTimestamp) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}

	path := d.File.Name()

	var atime, mtime time.Time

	// 为了处理 NoChange，我们需要先获取文件的当前时间戳
	if data_access_timestamp.NoChange != nil || data_modification_timestamp.NoChange != nil {
		info, err := os.Stat(path)
		if err != nil {
			return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
		}

		atime = info.ModTime()
		mtime = info.ModTime()
		if time, err := GetATime(info); err == nil {
			atime = time
		}
	}

	if data_access_timestamp.Timestamp != nil {
		atime = time.Unix(int64(data_access_timestamp.Timestamp.Seconds), int64(data_access_timestamp.Timestamp.Nanoseconds))
	} else if data_access_timestamp.Now != nil {
		atime = time.Now()
	}

	if data_modification_timestamp.Timestamp != nil {
		mtime = time.Unix(int64(data_modification_timestamp.Timestamp.Seconds), int64(data_modification_timestamp.Timestamp.Nanoseconds))
	} else if data_modification_timestamp.Now != nil {
		mtime = time.Now()
	}

	err := os.Chtimes(path, atime, mtime)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}
func (i *typesImpl) Read(ctx context.Context, this Descriptor, length Filesize, offset Filesize) witgo.Result[witgo.Tuple[[]byte, bool], ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Tuple[[]byte, bool], ErrorCode](ErrorCodeBadDescriptor)
	}
	buf := make([]byte, length)
	n, err := d.File.ReadAt(buf, int64(offset))
	endOfFile := err == io.EOF
	if err != nil && err != io.EOF {
		return witgo.Err[witgo.Tuple[[]byte, bool], ErrorCode](mapOsError(err))
	}
	result := witgo.Tuple[[]byte, bool]{F0: buf[:n], F1: endOfFile}
	return witgo.Ok[witgo.Tuple[[]byte, bool], ErrorCode](result)
}

func (i *typesImpl) Write(ctx context.Context, this Descriptor, buffer []byte, offset Filesize) witgo.Result[Filesize, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[Filesize, ErrorCode](ErrorCodeBadDescriptor)
	}
	n, err := d.File.WriteAt(buffer, int64(offset))
	if err != nil {
		return witgo.Err[Filesize, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[Filesize, ErrorCode](Filesize(n))
}

func (i *typesImpl) ReadDirectory(ctx context.Context, this Descriptor) witgo.Result[DirectoryEntryStream, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[DirectoryEntryStream, ErrorCode](ErrorCodeBadDescriptor)
	}

	entries, err := os.ReadDir(d.File.Name())
	if err != nil {
		return witgo.Err[DirectoryEntryStream, ErrorCode](mapOsError(err))
	}

	streamState := &filesystem.DirectoryEntryStreamState{
		Entries: entries,
		Index:   0,
	}
	handle := i.host.DirectoryEntryStreamManager().Add(streamState)
	return witgo.Ok[DirectoryEntryStream, ErrorCode](handle)
}

func (i *typesImpl) Sync(ctx context.Context, this Descriptor) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	err := d.File.Sync()
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) CreateDirectoryAt(ctx context.Context, this Descriptor, path string) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}

	fullPath := filepath.Join(d.File.Name(), path)

	// 默认权限 0755
	err := os.Mkdir(fullPath, 0755)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) Stat(ctx context.Context, this Descriptor) witgo.Result[DescriptorStat, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[DescriptorStat, ErrorCode](ErrorCodeBadDescriptor)
	}
	info, err := d.File.Stat()
	if err != nil {
		return witgo.Err[DescriptorStat, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[DescriptorStat, ErrorCode](goFileInfoToDescriptorStat(info))
}

func (i *typesImpl) StatAt(ctx context.Context, this Descriptor, pathFlags PathFlags, path string) witgo.Result[DescriptorStat, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[DescriptorStat, ErrorCode](ErrorCodeBadDescriptor)
	}

	// 警告：这是一个简化的、不安全的实现，将在下一步中被沙箱化路径解析替换。
	fullPath := filepath.Join(d.File.Name(), path)

	var info fs.FileInfo
	var err error

	if pathFlags.SymlinkFollow {
		// os.Stat 会跟随符号链接
		info, err = os.Stat(fullPath)
	} else {
		// os.Lstat 不会跟随符号链接
		info, err = os.Lstat(fullPath)
	}

	if err != nil {
		return witgo.Err[DescriptorStat, ErrorCode](mapOsError(err))
	}

	return witgo.Ok[DescriptorStat, ErrorCode](goFileInfoToDescriptorStat(info))
}

func (i *typesImpl) SetTimesAt(ctx context.Context, this Descriptor, pathFlags PathFlags, path string, data_access_timestamp NewTimestamp, data_modification_timestamp NewTimestamp) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}

	fullPath := filepath.Join(d.File.Name(), path)

	atime := time.Now() // 默认
	mtime := time.Now() // 默认

	// WIT 定义中没有 "don't change" 的选项，但 os.Chtimes 需要两个时间。
	// 我们用 "now" 作为默认值。
	if data_access_timestamp.Timestamp != nil {
		atime = time.Unix(int64(data_access_timestamp.Timestamp.Seconds), int64(data_access_timestamp.Timestamp.Nanoseconds))
	}
	if data_modification_timestamp.Timestamp != nil {
		mtime = time.Unix(int64(data_modification_timestamp.Timestamp.Seconds), int64(data_modification_timestamp.Timestamp.Nanoseconds))
	}

	err := os.Chtimes(fullPath, atime, mtime)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}

	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) LinkAt(ctx context.Context, this Descriptor, oldPathFlags PathFlags, oldPath string, newDescriptor Descriptor, newPath string) witgo.Result[witgo.Unit, ErrorCode] {
	oldDir, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	newDir, ok := i.host.FilesystemManager().Get(newDescriptor)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}

	oldFullPath := filepath.Join(oldDir.File.Name(), oldPath)
	newFullPath := filepath.Join(newDir.File.Name(), newPath)

	err := os.Link(oldFullPath, newFullPath)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) OpenAt(ctx context.Context, this Descriptor, pathFlags PathFlags, path string, openFlags OpenFlags, flags DescriptorFlags) witgo.Result[Descriptor, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[Descriptor, ErrorCode](ErrorCodeBadDescriptor)
	}

	fullPath := filepath.Join(d.File.Name(), path)

	var osFlags int
	if flags.Read && flags.Write {
		osFlags |= os.O_RDWR
	} else if flags.Write {
		osFlags |= os.O_WRONLY
	} else {
		osFlags |= os.O_RDONLY
	}

	if openFlags.Create {
		osFlags |= os.O_CREATE
	}
	if openFlags.Exclusive {
		osFlags |= os.O_EXCL
	}
	if openFlags.Truncate {
		osFlags |= os.O_TRUNC
	}

	// 默认权限 0644
	file, err := os.OpenFile(fullPath, osFlags, 0644)
	if err != nil {
		return witgo.Err[Descriptor, ErrorCode](mapOsError(err))
	}

	newDesc := &filesystem.Descriptor{
		File: file,
		Path: path,
	}
	handle := i.host.FilesystemManager().Add(newDesc)
	return witgo.Ok[Descriptor, ErrorCode](handle)
}

func (i *typesImpl) ReadlinkAt(ctx context.Context, this Descriptor, path string) witgo.Result[string, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[string, ErrorCode](ErrorCodeBadDescriptor)
	}
	fullPath := filepath.Join(d.File.Name(), path)

	target, err := os.Readlink(fullPath)
	if err != nil {
		return witgo.Err[string, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[string, ErrorCode](target)
}

func (i *typesImpl) RemoveDirectoryAt(ctx context.Context, this Descriptor, path string) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	fullPath := filepath.Join(d.File.Name(), path)

	// os.Remove 会删除空目录，但为了精确对应 rmdir，我们使用 syscall
	err := syscall.Rmdir(fullPath)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) RenameAt(ctx context.Context, this Descriptor, oldPath string, newDescriptor Descriptor, newPath string) witgo.Result[witgo.Unit, ErrorCode] {
	oldDir, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	newDir, ok := i.host.FilesystemManager().Get(newDescriptor)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}

	oldFullPath := filepath.Join(oldDir.File.Name(), oldPath)
	newFullPath := filepath.Join(newDir.File.Name(), newPath)

	err := os.Rename(oldFullPath, newFullPath)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) SymlinkAt(ctx context.Context, this Descriptor, oldPath string, newPath string) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	newFullPath := filepath.Join(d.File.Name(), newPath)

	err := os.Symlink(oldPath, newFullPath)
	if err != nil {
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) UnlinkFileAt(ctx context.Context, this Descriptor, path string) witgo.Result[witgo.Unit, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeBadDescriptor)
	}
	fullPath := filepath.Join(d.File.Name(), path)

	err := os.Remove(fullPath) // os.Remove 可以删除文件或空目录
	if err != nil {
		// 检查是否是目录，以返回正确的错误码
		if stat, statErr := os.Stat(fullPath); statErr == nil && stat.IsDir() {
			return witgo.Err[witgo.Unit, ErrorCode](ErrorCodeIsDirectory)
		}
		return witgo.Err[witgo.Unit, ErrorCode](mapOsError(err))
	}
	return witgo.Ok[witgo.Unit, ErrorCode](witgo.Unit{})
}

func (i *typesImpl) IsSameObject(ctx context.Context, this Descriptor, other Descriptor) bool {
	d1, ok1 := i.host.FilesystemManager().Get(this)
	d2, ok2 := i.host.FilesystemManager().Get(other)
	if !ok1 || !ok2 {
		return false
	}

	info1, err1 := d1.File.Stat()
	info2, err2 := d2.File.Stat()
	if err1 != nil || err2 != nil {
		return false
	}

	return os.SameFile(info1, info2)
}

func (i *typesImpl) MetadataHash(ctx context.Context, this Descriptor) witgo.Result[MetadataHashValue, ErrorCode] {
	return witgo.Err[MetadataHashValue, ErrorCode](ErrorCodeUnsupported)
}

func (i *typesImpl) MetadataHashAt(ctx context.Context, this Descriptor, pathFlags PathFlags, path string) witgo.Result[MetadataHashValue, ErrorCode] {
	return witgo.Err[MetadataHashValue, ErrorCode](ErrorCodeUnsupported)
}

func (i *typesImpl) DropDirectoryEntryStream(_ context.Context, handle DirectoryEntryStream) {
	i.host.DirectoryEntryStreamManager().Remove(handle)
}

func (i *typesImpl) ReadDirectoryEntry(_ context.Context, this DirectoryEntryStream) witgo.Result[witgo.Option[DirectoryEntry], ErrorCode] {
	stream, ok := i.host.DirectoryEntryStreamManager().Get(this)
	if !ok {
		return witgo.Err[witgo.Option[DirectoryEntry], ErrorCode](ErrorCodeBadDescriptor)
	}

	if stream.Index >= len(stream.Entries) {
		// 没有更多条目了，返回 None
		return witgo.Ok[witgo.Option[DirectoryEntry], ErrorCode](witgo.None[DirectoryEntry]())
	}

	entry := stream.Entries[stream.Index]
	stream.Index++

	info, err := entry.Info()
	if err != nil {
		return witgo.Err[witgo.Option[DirectoryEntry], ErrorCode](mapOsError(err))
	}

	dirEntry := DirectoryEntry{
		Type: goModeToDescriptorType(info.Mode()),
		Name: entry.Name(),
	}

	return witgo.Ok[witgo.Option[DirectoryEntry], ErrorCode](witgo.Some(dirEntry))
}

func (i *typesImpl) FilesystemErrorCode(ctx context.Context, err WasiError) witgo.Option[ErrorCode] {
	// 这个函数用于将 wasi:io/error 向 wasi:filesystem/error-code "向下转型"
	// 我们需要一种方法来存储原始的 os/syscall 错误
	if e, ok := i.host.ErrorManager().Get(err); ok {
		// 检查 e 是否是我们可以映射的错误类型
		if code := mapOsError(e); code != ErrorCodeUnsupported {
			return witgo.Some(code)
		}
	}
	return witgo.None[ErrorCode]()
}

// --- Helper: sectionWriter for WriteViaStream ---
type sectionWriter struct {
	w      *os.File
	offset int64
}

func (s *sectionWriter) Write(p []byte) (n int, err error) {
	n, err = s.w.WriteAt(p, s.offset)
	s.offset += int64(n)
	return
}

type OsFileFlusher struct {
	w interface{ Sync() error }
}

func (f *OsFileFlusher) Flush() error {
	return f.w.Sync()
}
