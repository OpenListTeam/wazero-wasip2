package http

import (
	"io"
	"net/http"
	"sync/atomic"
	"time"

	manager_io "github.com/foxxorcat/wazero-wasip2/manager/io"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

// Fields 代表 HTTP 头部或尾部。
type Fields = http.Header

// IncomingRequest 代表一个由 Host 接收的、传递给 Guest 的 HTTP 请求的内部表示。
type IncomingRequest struct {
	Request *http.Request

	Method    string
	Path      string
	Query     string
	Scheme    *string
	Authority *string
	Headers   uint32

	Body io.ReadCloser

	// BodyHandle 用于 Guest 端消费 Body
	BodyHandle uint32
}

// OutgoingRequest 代表一个由 guest 构建的出站 HTTP 请求。
type OutgoingRequest struct {
	// 在handle调用后设置
	Request *http.Request

	Method    string
	Scheme    *string
	Authority *string
	Path      string
	Headers   Fields

	Body io.Reader

	// 这里只是引用资源，不属于OutgoingRequest生命周期管理
	// BodyWriter 用于在 Host 端写入 Guest 提供的数据
	BodyWriter *io.PipeWriter
	BodyHandle uint32 // 指向 outgoing-body 资源的句柄

	// 消耗标记
	Consumed atomic.Bool
}

func (o *OutgoingRequest) Close() error {
	if o.Body != nil {
		if closer, ok := o.Body.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

// IncomingResponse 代表一个已到达的、由 Host 接收的 HTTP 响应。
type IncomingResponse struct {
	Response *http.Response

	StatusCode int
	Headers    Fields

	Body       *IncomingBody
	BodyHandle uint32 // 指向 incoming-body 的句柄
	// 消耗标记
	Consumed atomic.Bool
}

// OutgoingResponse 代表一个由 Guest 构建的出站 HTTP 响应。
type OutgoingResponse struct {
	Response http.ResponseWriter

	StatusCode int
	Headers    Fields

	Body io.Reader

	// 这里只是引用资源，不属于OutgoingResponse生命周期管理
	BodyWriter *io.PipeWriter
	BodyHandle uint32

	// 消耗标记
	Consumed atomic.Bool
}

func (o *OutgoingResponse) Close() error {
	if o.Body != nil {
		if closer, ok := o.Body.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

// ResponseOutparam 是一个一次性的句柄，用于让 Guest 设置对 IncomingRequest 的响应。
type ResponseOutparam struct {
	// 当 Guest 调用 response-outparam.set 时，结果会通过这个 channel 发送。
	// Result 包含一个 OutgoingResponse 的句柄或一个 ErrorCode。
	ResultChan chan<- any
}

// IncomingBody 代表一个入站的 HTTP Body。
type IncomingBody struct {
	// 因为go http 的限制Body和Stream 生命周期统一管理
	StreamHandle uint32 // 指向 input-stream 的句柄
	Stream       io.Reader

	// 可选方法
	GetTrailers func() (trailers Fields)

	// 消耗标记
	Consumed atomic.Bool
}

func (o *IncomingBody) Close() error {
	if o.Stream != nil {
		if closer, ok := o.Stream.(io.Closer); ok {
			closer.Close()
		}
	}
	return nil
}

// OutgoingBody 代表一个出站的 HTTP Body。
type OutgoingBody struct {
	// 因为go http 的限制Body和Stream 生命周期统一管理
	OutputStreamHandle uint32
	BodyWriter         *io.PipeWriter

	// 可选方法
	SetTrailers func(trailers Fields) error

	ContentLength *uint64
	BytesWritten  atomic.Uint64

	// 消耗标记
	Consumed atomic.Bool
}

func (o *OutgoingBody) Close() error {
	if o.BodyWriter != nil {
		o.BodyWriter.CloseWithError(io.EOF)
	}
	return nil
}

// FutureTrailers 代表一个尚未到达的 HTTP Trailers。
type FutureTrailers struct {
	Pollable *manager_io.ChannelPollable
	Result   ResultTrailers
	Consumed atomic.Bool
}

type ResultTrailers struct {
	Trailers Fields
	Err      error
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
	Pollable *manager_io.ChannelPollable
	Consumed atomic.Bool
	Result   Result
}

// Result 是一个内部类型，用于在 goroutine 之间传递 HTTP 请求的结果。
type Result struct {
	// ResponseHandle uint32 // 指向 IncomingResponse 的句柄
	Response *http.Response
	Err      error // 或一个 Go 的 error
}

// FieldsManager 使用通用 ResourceManager 来管理 Fields 资源。
type FieldsManager = witgo.ResourceManager[Fields]

func NewFieldsManager() *FieldsManager {
	return witgo.NewResourceManager[Fields](nil)
}

// HTTPManager 是所有 HTTP 相关资源的总管理器。
type HTTPManager struct {
	Fields  *FieldsManager
	Streams *manager_io.StreamManager
	Poll    *manager_io.PollManager

	Options          *witgo.ResourceManager[*RequestOptions]
	OutgoingRequests *witgo.ResourceManager[*OutgoingRequest]
	Futures          *witgo.ResourceManager[*FutureIncomingResponse]
	Responses        *witgo.ResourceManager[*IncomingResponse]
	Bodies           *witgo.ResourceManager[*OutgoingBody]
	FutureTrailers   *witgo.ResourceManager[*FutureTrailers]

	IncomingRequests  *witgo.ResourceManager[*IncomingRequest]
	ResponseOutparams *witgo.ResourceManager[*ResponseOutparam]
	OutgoingResponses *witgo.ResourceManager[*OutgoingResponse]
	IncomingBodies    *witgo.ResourceManager[*IncomingBody]
}

func NewHTTPManager(sm *manager_io.StreamManager, poll *manager_io.PollManager) *HTTPManager {
	return &HTTPManager{
		Fields:  NewFieldsManager(),
		Streams: sm,
		Poll:    poll,

		// guest -> host
		Options: witgo.NewResourceManager[*RequestOptions](nil),
		OutgoingRequests: witgo.NewResourceManager[*OutgoingRequest](func(resource *OutgoingRequest) {
			resource.Close()
		}),
		Futures:   witgo.NewResourceManager[*FutureIncomingResponse](nil),
		Responses: witgo.NewResourceManager[*IncomingResponse](nil),
		Bodies: witgo.NewResourceManager[*OutgoingBody](func(resource *OutgoingBody) {
			resource.Close()

			// NOTE: 为了防止忘记关闭，这里的生命周期和Stream绑定
			if resource.BodyWriter != nil {
				resource.BodyWriter.CloseWithError(io.ErrShortWrite)
				sm.Remove(resource.OutputStreamHandle)
			}
		}),
		FutureTrailers: witgo.NewResourceManager[*FutureTrailers](nil),

		IncomingRequests: witgo.NewResourceManager[*IncomingRequest](func(resource *IncomingRequest) {
			resource.Body.Close()
		}),
		ResponseOutparams: witgo.NewResourceManager[*ResponseOutparam](func(resource *ResponseOutparam) {
			close(resource.ResultChan)
		}),
		OutgoingResponses: witgo.NewResourceManager[*OutgoingResponse](func(resource *OutgoingResponse) {
			resource.Close()
		}),
		IncomingBodies: witgo.NewResourceManager[*IncomingBody](func(resource *IncomingBody) {
			resource.Close()

			// NOTE: 为了防止忘记关闭，这里的生命周期和Stream绑定
			if resource.Stream != nil {
				sm.Remove(resource.StreamHandle)
			}
		}),
	}
}

func (hm *HTTPManager) NewOutgoingBody(contentLength *uint64, setTrailers func(trailers Fields) error) (bodyHandle uint32, bodyReader *io.PipeReader, bodyWriter *io.PipeWriter) {
	pr, pw := io.Pipe()

	body := &OutgoingBody{
		// OutputStreamHandle: 0, // 使用时设置
		BodyWriter:    pw,
		SetTrailers:   setTrailers,
		ContentLength: contentLength,
	}

	bodyHandle = hm.Bodies.Add(body)
	return bodyHandle, pr, pw
}
