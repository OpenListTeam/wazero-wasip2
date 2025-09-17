package v0_2

import (
	io_v0_2 "wazero-wasip2/wasip2/io/v0_2"
)

// Pollable 从 wasi:io 导入
type Pollable = io_v0_2.Pollable

// --- monotonic-clock types ---
type Instant = uint64
type Duration = uint64

// --- wall-clock types ---
type Datetime struct {
	Seconds     uint64
	Nanoseconds uint32
}
