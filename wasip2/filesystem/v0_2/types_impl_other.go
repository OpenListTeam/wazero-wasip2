//go:build !unix && !windows

package v0_2

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"time"
	witgo "wazero-wasip2/wit-go"
)

func (i *typesImpl) GetFlags(ctx context.Context, this Descriptor) witgo.Result[DescriptorFlags, ErrorCode] {
	return witgo.Err[DescriptorFlags, ErrorCode](ErrorCodeUnsupported)
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

	// For non-Unix platforms, we provide best-effort information.
	stat.DataAccessTimestamp = witgo.None[Datetime]()
	stat.StatusChangeTimestamp = witgo.None[Datetime]()
	// Link count might not be available.
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

	return ErrorCodeUnsupported
}

func GetATime(info os.FileInfo) (time.Time, error) {
	return time.Time{}, errors.New("GetATime not supported on this platform")
}
