//go:build windows

package v0_2

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"
	"time"
	witgo "wazero-wasip2/wit-go"

	"golang.org/x/sys/windows"
)

// GetFlags now provides a best-effort implementation for Windows by checking the file's read-only attribute.
func (i *typesImpl) GetFlags(ctx context.Context, this Descriptor) witgo.Result[DescriptorFlags, ErrorCode] {
	d, ok := i.host.FilesystemManager().Get(this)
	if !ok {
		return witgo.Err[DescriptorFlags, ErrorCode](ErrorCodeBadDescriptor)
	}

	info, err := d.File.Stat()
	if err != nil {
		return witgo.Err[DescriptorFlags, ErrorCode](mapOsError(err))
	}

	var wasiFlags DescriptorFlags
	wasiFlags.Read = true // Assume readable by default on Windows unless write-only is specified.

	// The most reliable flag we can check on Windows is the read-only attribute.
	if info.Mode()&0200 == 0 { // Check for write permission bit.
		// If the file is not writable, we can infer it's for reading only.
		wasiFlags.Write = false
	} else {
		wasiFlags.Write = true
	}

	// Windows doesn't directly map to the sync flags in the same way as Unix.
	// This remains a best-effort mapping.
	wasiFlags.FileIntegritySync = false
	wasiFlags.DataIntegritySync = false
	wasiFlags.RequestedWriteSync = false

	return witgo.Ok[DescriptorFlags, ErrorCode](wasiFlags)
}

// timeToDatetime has been corrected to prevent precision loss during conversion.
func timeToDatetime(ft syscall.Filetime) Datetime {
	// Correctly convert FILETIME to Unix time to avoid overflow and precision loss.
	// A FILETIME is the number of 100-nanosecond intervals since January 1, 1601.
	// We convert it to a Go time.Time object first.
	t := time.Unix(0, ft.Nanoseconds()).UTC()
	return Datetime{
		Seconds:     uint64(t.Unix()),
		Nanoseconds: uint32(t.Nanosecond()),
	}
}

func goFileInfoToDescriptorStat(info fs.FileInfo) DescriptorStat {
	var stat DescriptorStat
	stat.Type = goModeToDescriptorType(info.Mode())
	stat.Size = Filesize(info.Size())
	modTime := info.ModTime()
	stat.DataModificationTimestamp = witgo.Some(Datetime{
		Seconds:     uint64(modTime.Unix()),
		Nanoseconds: uint32(modTime.Nanosecond()),
	})

	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		stat.DataAccessTimestamp = witgo.Some(timeToDatetime(sys.LastAccessTime))
		// On Windows, ctime (status change time) is not maintained. LastWriteTime is the closest equivalent.
		stat.StatusChangeTimestamp = witgo.Some(timeToDatetime(sys.LastWriteTime))
	} else {
		stat.DataAccessTimestamp = witgo.None[Datetime]()
		stat.StatusChangeTimestamp = witgo.None[Datetime]()
	}

	// Link count requires a more complex API call (GetFileInformationByHandle)
	// which is not readily available from os.FileInfo. Defaulting to 1 is a reasonable fallback.
	stat.LinkCount = 1

	return stat
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
		return DescriptorTypeCharacterDevice
	case mode&fs.ModeNamedPipe != 0:
		return DescriptorTypeFifo
	case mode&fs.ModeSocket != 0:
		return DescriptorTypeSocket
	default:
		return DescriptorTypeUnknown
	}
}

// mapOsError is now corrected with the proper error constants from the x/sys/windows package.
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
		case windows.ERROR_ACCESS_DENIED:
			return ErrorCodeAccess
		case windows.ERROR_ALREADY_EXISTS, windows.ERROR_FILE_EXISTS:
			return ErrorCodeExist
		case windows.ERROR_FILE_NOT_FOUND, windows.ERROR_PATH_NOT_FOUND:
			return ErrorCodeNoEntry
		case windows.ERROR_DIR_NOT_EMPTY:
			return ErrorCodeNotEmpty
		case windows.ERROR_INVALID_HANDLE:
			return ErrorCodeBadDescriptor
		case windows.ERROR_INVALID_PARAMETER:
			return ErrorCodeInvalid
		case windows.ERROR_SHARING_VIOLATION:
			return ErrorCodeBusy
		case windows.ERROR_NOT_SUPPORTED:
			return ErrorCodeUnsupported
		case windows.ERROR_DISK_FULL:
			return ErrorCodeInsufficientSpace
		case windows.ERROR_BROKEN_PIPE:
			return ErrorCodePipe
		case windows.ERROR_NOT_A_REPARSE_POINT: // Not a symlink
			return ErrorCodeInvalid
		case windows.ERROR_DIRECTORY: // e.g. trying to unlink a directory as a file
			return ErrorCodeIsDirectory
		}
	}
	return ErrorCodeUnsupported
}

func GetATime(info os.FileInfo) (time.Time, error) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, syscall.EINVAL
	}
	return time.Unix(0, data.LastAccessTime.Nanoseconds()), nil
}
