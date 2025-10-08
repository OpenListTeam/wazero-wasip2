package v0_2

import (
	io_v0_2 "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
)

// --- Imported Types ---
type InputStream = io_v0_2.InputStream
type OutputStream = io_v0_2.OutputStream
type Pollable = io_v0_2.Pollable
type WasiError = io_v0_2.Error

// --- Base Types ---
type ClientHandshake = uint32
type ClientConnection = uint32
type FutureClientStreams = uint32
