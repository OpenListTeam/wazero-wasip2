package http

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	manager_io "wazero-wasip2/internal/io"
	witgo "wazero-wasip2/wit-go"
)

// Fields 代表 HTTP 头部或尾部。
type Fields map[string][]string

// IncomingRequest 代表一个由 Host 接收的、传递给 Guest 的 HTTP 请求的内部表示。
type IncomingRequest struct {
	Method    string
	Path      string
	Query     string
	Scheme    *string
	Authority *string
	Headers   uint32
	Body      io.ReadCloser
	// BodyHandle 用于 Guest 端消费 Body
	BodyHandle uint32
}

// OutgoingRequest 代表一个由 guest 构建的出站 HTTP 请求。
type OutgoingRequest struct {
	Method    string
	Scheme    *string
	Authority *string
	Path      string
	Headers   uint32 // 指向 Fields 资源的句柄
	Body      io.Reader
	// BodyWriter 用于在 Host 端写入 Guest 提供的数据
	BodyWriter *io.PipeWriter
	BodyHandle uint32 // 指向 outgoing-body 资源的句柄
}

// IncomingResponse 代表一个已到达的、由 Host 接收的 HTTP 响应。
type IncomingResponse struct {
	Response     *http.Response
	Headers      uint32
	BodyConsumed bool
	BodyHandle   uint32 // 指向 incoming-body 的句柄
}

// OutgoingResponse 代表一个由 Guest 构建的出站 HTTP 响应。
type OutgoingResponse struct {
	StatusCode int
	Headers    uint32
	Body       io.Reader
	BodyWriter *io.PipeWriter
	BodyHandle uint32
}

// ResponseOutparam 是一个一次性的句柄，用于让 Guest 设置对 IncomingRequest 的响应。
type ResponseOutparam struct {
	// 当 Guest 调用 response-outparam.set 时，结果会通过这个 channel 发送。
	// Result 包含一个 OutgoingResponse 的句柄或一个 ErrorCode。
	ResultChan chan<- any
}

// IncomingBody 代表一个入站的 HTTP Body。
type IncomingBody struct {
	Body         io.ReadCloser
	StreamHandle uint32 // 指向 input-stream 的句柄
	StreamTaken  bool   // 标记 stream 是否已经被取出
	Trailers     uint32 // 指向 future-trailers 的句柄
}

// OutgoingBody 代表一个出站的 HTTP Body。
type OutgoingBody struct {
	OutputStreamHandle uint32
	BodyWriter         *io.PipeWriter
	Request            uint32 // OutgoingRequest or OutgoingResponse
}

// FutureTrailers 代表一个尚未到达的 HTTP Trailers。
type FutureTrailers struct {
	ResultChan   chan ResultTrailers
	Result       atomic.Pointer[ResultTrailers]
	Consumed     atomic.Bool
	Pollable     chan struct{}
	PollableOnce sync.Once
}

type ResultTrailers struct {
	TrailersHandle uint32 // 指向 Fields (Trailers) 的句柄
	Err            error
}

// RequestOptions 存储了 wasi:http/types.request-options 的状态。
type RequestOptions struct {
	ConnectTimeout      *time.Duration
	FirstByteTimeout    *time.Duration
	BetweenBytesTimeout *time.Duration
}

// FutureIncomingResponse 代表一个尚未到达的 HTTP 响应。
// ResultChan 是实现异步的核心。
type FutureIncomingResponse struct {
	ResultChan   chan Result
	Result       atomic.Pointer[Result]
	Consumed     atomic.Bool
	Pollable     chan struct{}
	PollableOnce sync.Once
}

// Result 是一个内部类型，用于在 goroutine 之间传递 HTTP 请求的结果。
type Result struct {
	ResponseHandle uint32 // 指向 IncomingResponse 的句柄
	Err            error  // 或一个 Go 的 error
}

// FieldsManager 使用通用 ResourceManager 来管理 Fields 资源。
type FieldsManager = witgo.ResourceManager[Fields]

func NewFieldsManager() *FieldsManager {
	return witgo.NewResourceManager[Fields]()
}

// HTTPManager 是所有 HTTP 相关资源的总管理器。
type HTTPManager struct {
	Fields            *FieldsManager
	Streams           *manager_io.StreamManager
	Poll              *manager_io.PollManager
	IncomingRequests  *witgo.ResourceManager[*IncomingRequest]
	OutgoingRequests  *witgo.ResourceManager[*OutgoingRequest]
	Responses         *witgo.ResourceManager[*IncomingResponse]
	OutgoingResponses *witgo.ResourceManager[*OutgoingResponse]
	ResponseOutparams *witgo.ResourceManager[*ResponseOutparam]
	Bodies            *witgo.ResourceManager[*OutgoingBody]
	Futures           *witgo.ResourceManager[*FutureIncomingResponse]
	IncomingBodies    *witgo.ResourceManager[*IncomingBody]
	FutureTrailers    *witgo.ResourceManager[*FutureTrailers]
	Options           *witgo.ResourceManager[*RequestOptions]
}

func NewHTTPManager(sm *manager_io.StreamManager, poll *manager_io.PollManager) *HTTPManager {
	return &HTTPManager{
		Fields:            NewFieldsManager(),
		Streams:           sm,
		Poll:              poll,
		IncomingRequests:  witgo.NewResourceManager[*IncomingRequest](),
		OutgoingRequests:  witgo.NewResourceManager[*OutgoingRequest](),
		Responses:         witgo.NewResourceManager[*IncomingResponse](),
		OutgoingResponses: witgo.NewResourceManager[*OutgoingResponse](),
		ResponseOutparams: witgo.NewResourceManager[*ResponseOutparam](),
		Bodies:            witgo.NewResourceManager[*OutgoingBody](),
		Futures:           witgo.NewResourceManager[*FutureIncomingResponse](),
		IncomingBodies:    witgo.NewResourceManager[*IncomingBody](),
		FutureTrailers:    witgo.NewResourceManager[*FutureTrailers](),
		Options:           witgo.NewResourceManager[*RequestOptions](),
	}
}
