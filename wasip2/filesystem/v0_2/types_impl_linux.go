//go:build linux

package v0_2

import (
	"io/fs"
	"os"
	"syscall"
	"time"
	witgo "wazero-wasip2/wit-go"
)

func goFileInfoToDescriptorStat(info fs.FileInfo) DescriptorStat {
	var stat DescriptorStat
	stat.Type = goModeToDescriptorType(info.Mode())
	stat.Size = Filesize(info.Size())
	modTime := info.ModTime()
	stat.DataModificationTimestamp = witgo.Some(Datetime{
		Seconds:     uint64(modTime.Unix()),
		Nanoseconds: uint32(modTime.Nanosecond()),
	})

	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		stat.LinkCount = uint64(sys.Nlink)
		stat.DataAccessTimestamp = witgo.Some(timeToDatetime(sys.Atim))
		stat.StatusChangeTimestamp = witgo.Some(timeToDatetime(sys.Ctim))
	} else {
		stat.DataAccessTimestamp = witgo.None[Datetime]()
		stat.StatusChangeTimestamp = witgo.None[Datetime]()
	}
	return stat
}

func GetATime(info os.FileInfo) (time.Time, error) {
	// 从FileInfo中获取底层的syscall.Stat_t
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, syscall.EINVAL
	}

	// 转换为time.Time
	return time.Unix(stat.Atim.Sec, int64(stat.Atim.Nsec)), nil
}
