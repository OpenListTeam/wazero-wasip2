//go:build unix

package v0_2

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"
	witgo "wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *typesImpl) GetFlags(ctx context.Context, this Descriptor) witgo.Result[DescriptorFlags, ErrorCode] {
	// 在类Unix系统上，可以使用 fcntl 获取 flags。
	// 这需要平台特定的实现。
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[DescriptorFlags, ErrorCode](ErrorCodeBadDescriptor)
	}

	flags, err := unix.FcntlInt(d.File.Fd(), unix.F_GETFL, 0)
	if err != nil {
		return witgo.Err[DescriptorFlags, ErrorCode](mapOsError(err))
	}

	// 将 os flags 映射到 wasi flags
	var wasiFlags DescriptorFlags
	if flags&os.O_RDWR != 0 {
		wasiFlags.Read = true
		wasiFlags.Write = true
	} else if flags&os.O_WRONLY != 0 {
		wasiFlags.Write = true
	} else { // O_RDONLY
		wasiFlags.Read = true
	}
	// 其他 flags (O_SYNC, O_DSYNC) 的映射可以后续添加

	return witgo.Ok[DescriptorFlags, ErrorCode](wasiFlags)
}

func timeToDatetime(ts syscall.Timespec) Datetime {
	return Datetime{
		Seconds:     uint64(ts.Sec),
		Nanoseconds: uint32(ts.Nsec),
	}
}

func goModeToDescriptorType(mode fs.FileMode) DescriptorType {
	switch {
	case mode.IsRegular():
		return DescriptorTypeRegularFile
	case mode.IsDir():
		return DescriptorTypeDirectory
	case mode&fs.ModeSymlink != 0:
		return DescriptorTypeSymbolicLink
	case mode&fs.ModeDevice != 0:
		if mode&fs.ModeCharDevice != 0 {
			return DescriptorTypeCharacterDevice
		}
		return DescriptorTypeBlockDevice
	case mode&fs.ModeNamedPipe != 0:
		return DescriptorTypeFifo
	case mode&fs.ModeSocket != 0:
		return DescriptorTypeSocket
	default:
		return DescriptorTypeUnknown
	}
}

func mapOsError(err error) ErrorCode {
	if err == nil {
		return 0
	}
	if errors.Is(err, fs.ErrPermission) {
		return ErrorCodeAccess
	}
	if errors.Is(err, fs.ErrExist) {
		return ErrorCodeExist
	}
	if errors.Is(err, fs.ErrNotExist) {
		return ErrorCodeNoEntry
	}
	if errors.Is(err, fs.ErrInvalid) {
		return ErrorCodeInvalid
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES:
			return ErrorCodeAccess
		case syscall.EAGAIN:
			return ErrorCodeWouldBlock
		case syscall.EBADF:
			return ErrorCodeBadDescriptor
		case syscall.EBUSY:
			return ErrorCodeBusy
		case syscall.EEXIST:
			return ErrorCodeExist
		case syscall.EFBIG:
			return ErrorCodeFileTooLarge
		case syscall.EINTR:
			return ErrorCodeInterrupted
		case syscall.EINVAL:
			return ErrorCodeInvalid
		case syscall.EIO:
			return ErrorCodeIo
		case syscall.EISDIR:
			return ErrorCodeIsDirectory
		case syscall.ELOOP:
			return ErrorCodeLoop
		case syscall.EMLINK:
			return ErrorCodeTooManyLinks
		case syscall.ENAMETOOLONG:
			return ErrorCodeNameTooLong
		case syscall.ENODEV:
			return ErrorCodeNoDevice
		case syscall.ENOENT:
			return ErrorCodeNoEntry
		case syscall.ENOLCK:
			return ErrorCodeNoLock
		case syscall.ENOMEM:
			return ErrorCodeInsufficientMemory
		case syscall.ENOSPC:
			return ErrorCodeInsufficientSpace
		case syscall.ENOTDIR:
			return ErrorCodeNotDirectory
		case syscall.ENOTEMPTY:
			return ErrorCodeNotEmpty
		case syscall.ENOTSUP:
			return ErrorCodeUnsupported
		case syscall.ENXIO:
			return ErrorCodeNoSuchDevice
		case syscall.EOVERFLOW:
			return ErrorCodeOverflow
		case syscall.EPERM:
			return ErrorCodeNotPermitted
		case syscall.EPIPE:
			return ErrorCodePipe
		case syscall.EROFS:
			return ErrorCodeReadOnly
		case syscall.ESPIPE:
			return ErrorCodeInvalidSeek
		case syscall.ETXTBSY:
			return ErrorCodeTextFileBusy
		case syscall.EXDEV:
			return ErrorCodeCrossDevice
		}
	}
	return ErrorCodeUnsupported
}
