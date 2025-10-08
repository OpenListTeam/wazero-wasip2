package v0_2

import (
	wasip2_clocks "github.com/foxxorcat/wazero-wasip2/wasip2/clocks/v0_2"
	wasip2_io "github.com/foxxorcat/wazero-wasip2/wasip2/io/v0_2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

// 从 wasi:io 导入类型
type InputStream = wasip2_io.InputStream
type OutputStream = wasip2_io.OutputStream
type WasiError = wasip2_io.Error
type Pollable = wasip2_io.Pollable
type Duration = wasip2_clocks.Duration

type FieldKey = string
type FieldValue = []byte
type StatusCode = uint16

// HTTP 资源句柄
type Fields = uint32
type Headers = uint32
type IncomingRequest = uint32
type OutgoingRequest = uint32
type RequestOptions = uint32
type IncomingResponse = uint32
type OutgoingResponse = uint32
type IncomingBody = uint32
type FutureTrailers = uint32
type OutgoingBody = uint32
type FutureIncomingResponse = uint32
type Trailers = uint32
type ResponseOutparam = uint32

// Method 对应 WIT 中的 method variant
type Method struct {
	Get     *witgo.Unit `wit:"case(0)"`
	Head    *witgo.Unit `wit:"case(1)"`
	Post    *witgo.Unit `wit:"case(2)"`
	Put     *witgo.Unit `wit:"case(3)"`
	Delete  *witgo.Unit `wit:"case(4)"`
	Connect *witgo.Unit `wit:"case(5)"`
	Options *witgo.Unit `wit:"case(6)"`
	Trace   *witgo.Unit `wit:"case(7)"`
	Patch   *witgo.Unit `wit:"case(8)"`
	Other   *string     `wit:"case(9)"`
}

// Scheme 对应 WIT 中的 scheme variant
type Scheme struct {
	HTTP  *witgo.Unit `wit:"case(0)"`
	HTTPS *witgo.Unit `wit:"case(1)"`
	Other *string     `wit:"case(2)"`
}

// DNSErrorPayload 对应 WIT 中的 DNS-error-payload record
type DNSErrorPayload struct {
	Rcode    witgo.Option[string]
	InfoCode witgo.Option[uint16]
}

// TLSAlertReceivedPayload 对应 WIT 中的 TLS-alert-received-payload record
type TLSAlertReceivedPayload struct {
	AlertID      witgo.Option[uint8]
	AlertMessage witgo.Option[string]
}

// FieldSizePayload 对应 WIT 中的 field-size-payload record
type FieldSizePayload struct {
	FieldName witgo.Option[string]
	FieldSize witgo.Option[uint32]
}

// ErrorCode 对应 WIT 中的 error-code variant
type ErrorCode struct {
	DNSTimeout                     *witgo.Unit                     `wit:"case(0)"`
	DNSError                       *DNSErrorPayload                `wit:"case(1)"`
	DestinationNotFound            *witgo.Unit                     `wit:"case(2)"`
	DestinationUnavailable         *witgo.Unit                     `wit:"case(3)"`
	DestinationIPProhibited        *witgo.Unit                     `wit:"case(4)"`
	DestinationIPUnroutable        *witgo.Unit                     `wit:"case(5)"`
	ConnectionRefused              *witgo.Unit                     `wit:"case(6)"`
	ConnectionTerminated           *witgo.Unit                     `wit:"case(7)"`
	ConnectionTimeout              *witgo.Unit                     `wit:"case(8)"`
	ConnectionReadTimeout          *witgo.Unit                     `wit:"case(9)"`
	ConnectionWriteTimeout         *witgo.Unit                     `wit:"case(10)"`
	ConnectionLimitReached         *witgo.Unit                     `wit:"case(11)"`
	TLSProtocolError               *witgo.Unit                     `wit:"case(12)"`
	TLSCertificateError            *witgo.Unit                     `wit:"case(13)"`
	TLSAlertReceived               *TLSAlertReceivedPayload        `wit:"case(14)"`
	HTTPRequestDenied              *witgo.Unit                     `wit:"case(15)"`
	HTTPRequestLengthRequired      *witgo.Unit                     `wit:"case(16)"`
	HTTPRequestBodySize            *witgo.Option[uint64]           `wit:"case(17)"`
	HTTPRequestMethodInvalid       *witgo.Unit                     `wit:"case(18)"`
	HTTPRequestURIInvalid          *witgo.Unit                     `wit:"case(19)"`
	HTTPRequestURITooLong          *witgo.Unit                     `wit:"case(20)"`
	HTTPRequestHeaderSectionSize   *witgo.Option[uint32]           `wit:"case(21)"`
	HTTPRequestHeaderSize          *witgo.Option[FieldSizePayload] `wit:"case(22)"`
	HTTPRequestTrailerSectionSize  *witgo.Option[uint32]           `wit:"case(23)"`
	HTTPRequestTrailerSize         *FieldSizePayload               `wit:"case(24)"`
	HTTPResponseIncomplete         *witgo.Unit                     `wit:"case(25)"`
	HTTPResponseHeaderSectionSize  *witgo.Option[uint32]           `wit:"case(26)"`
	HTTPResponseHeaderSize         *FieldSizePayload               `wit:"case(27)"`
	HTTPResponseBodySize           *witgo.Option[uint64]           `wit:"case(28)"`
	HTTPResponseTrailerSectionSize *witgo.Option[uint32]           `wit:"case(29)"`
	HTTPResponseTrailerSize        *FieldSizePayload               `wit:"case(30)"`
	HTTPResponseTransferCoding     *witgo.Option[string]           `wit:"case(31)"`
	HTTPResponseContentCoding      *witgo.Option[string]           `wit:"case(32)"`
	HTTPResponseTimeout            *witgo.Unit                     `wit:"case(33)"`
	HTTPUpgradeFailed              *witgo.Unit                     `wit:"case(34)"`
	HTTPProtocolError              *witgo.Unit                     `wit:"case(35)"`
	LoopDetected                   *witgo.Unit                     `wit:"case(36)"`
	ConfigurationError             *witgo.Unit                     `wit:"case(37)"`
	InternalError                  *witgo.Option[string]           `wit:"case(38)"`
}

// HeaderError 对应 WIT 中的 header-error variant
type HeaderError struct {
	// 在fields中设置头部的操作中，使用的field-name或field-value存在语法无效的问题。
	InvalidSyntax *witgo.Unit `wit:"case(0)"`
	// 在尝试在fields中设置头部时，使用了被禁止的field-name
	Forbidden *witgo.Unit `wit:"case(1)"`
	// 由于fields是不可变的，因此不允许对其执行该操作
	Immutable *witgo.Unit `wit:"case(2)"`
}
