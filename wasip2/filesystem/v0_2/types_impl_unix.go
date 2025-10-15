//go:build unix

package v0_2

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"

	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"golang.org/x/sys/unix"
)

func (i *typesImpl) GetFlags(ctx context.Context, this Descriptor) witgo.Result[DescriptorFlags, ErrorCode] {
	// 在类Unix系统上，可以使用 fcntl 获取 flags。
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

	if flags&unix.O_DSYNC != 0 {
		wasiFlags.DataIntegritySync = true
	}
	if flags&unix.O_SYNC != 0 {
		wasiFlags.FileIntegritySync = true
		wasiFlags.RequestedWriteSync = true
	}

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
		case unix.EACCES:
			return ErrorCodeAccess
		case unix.EAGAIN:
			return ErrorCodeWouldBlock
		case unix.EBADF:
			return ErrorCodeBadDescriptor
		case unix.EBUSY:
			return ErrorCodeBusy
		case unix.EEXIST:
			return ErrorCodeExist
		case unix.EFBIG:
			return ErrorCodeFileTooLarge
		case unix.EINTR:
			return ErrorCodeInterrupted
		case unix.EINVAL:
			return ErrorCodeInvalid
		case unix.EIO:
			return ErrorCodeIo
		case unix.EISDIR:
			return ErrorCodeIsDirectory
		case unix.ELOOP:
			return ErrorCodeLoop
		case unix.EMLINK:
			return ErrorCodeTooManyLinks
		case unix.ENAMETOOLONG:
			return ErrorCodeNameTooLong
		case unix.ENODEV:
			return ErrorCodeNoDevice
		case unix.ENOENT:
			return ErrorCodeNoEntry
		case unix.ENOLCK:
			return ErrorCodeNoLock
		case unix.ENOMEM:
			return ErrorCodeInsufficientMemory
		case unix.ENOSPC:
			return ErrorCodeInsufficientSpace
		case unix.ENOTDIR:
			return ErrorCodeNotDirectory
		case unix.ENOTEMPTY:
			return ErrorCodeNotEmpty
		case unix.ENOTSUP:
			return ErrorCodeUnsupported
		case unix.ENXIO:
			return ErrorCodeNoSuchDevice
		case unix.EOVERFLOW:
			return ErrorCodeOverflow
		case unix.EPERM:
			return ErrorCodeNotPermitted
		case unix.EPIPE:
			return ErrorCodePipe
		case unix.EROFS:
			return ErrorCodeReadOnly
		case unix.ESPIPE:
			return ErrorCodeInvalidSeek
		case unix.ETXTBSY:
			return ErrorCodeTextFileBusy
		case unix.EXDEV:
			return ErrorCodeCrossDevice
		}
	}
	return ErrorCodeUnsupported
}
