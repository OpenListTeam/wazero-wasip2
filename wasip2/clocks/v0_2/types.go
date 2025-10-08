package v0_2

import (
	"time"

	io_v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
)

// Pollable 从 wasi:io 导入
type Pollable = io_v0_2.Pollable

// --- monotonic-clock types ---
type Instant = uint64
type Duration uint64

func (d Duration) ToTime() time.Time {
	return time.Unix(0, int64(d))
}

func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// --- wall-clock types ---
type Datetime struct {
	Seconds     uint64
	Nanoseconds uint32
}
