package v0_2

import (
	witgo "wazero-wasip2/wit-go"

	errors_v2_0 "wazero-wasip2/wasip2/error/v0_2"
)

type InputStream = uint32
type OutputStream = uint32
type Pollable = uint32

type WasiError = errors_v2_0.Error

type StreamError struct {
	// 当 case 为 'last-operation-failed' 时，此字段为非 nil。
	LastOperationFailed *WasiError `wit:"case(0)"`
	// 当 case 为 'closed' 时，此字段为非 nil。
	Closed *witgo.Unit `wit:"case(1)"`
}
