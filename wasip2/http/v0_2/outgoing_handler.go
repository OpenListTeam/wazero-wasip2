package v0_2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	gohttp "net/http"
	"strings"
	"time"
	"wazero-wasip2/internal/http"
	"wazero-wasip2/internal/streams"
	witgo "wazero-wasip2/wit-go"

	lru "github.com/hashicorp/golang-lru/v2"
)

type timeoutConfig struct {
	connect      time.Duration
	firstByte    time.Duration
	betweenBytes time.Duration
}

// outgoingHandlerImpl 封装了 wasi:http/outgoing-handler 的所有操作。
type outgoingHandlerImpl struct {
	hm          *http.HTTPManager
	clientCache *lru.Cache[timeoutConfig, *gohttp.Client]
}

func newOutgoingHandlerImpl(hm *http.HTTPManager) *outgoingHandlerImpl {
	clientCache, _ := lru.New[timeoutConfig, *gohttp.Client](32)
	return &outgoingHandlerImpl{hm, clientCache}
}

func (i *outgoingHandlerImpl) getClient(options witgo.Option[RequestOptions]) *gohttp.Client {
	if options.Some != nil {
		var cfg timeoutConfig
		if opts, ok := i.hm.Options.Get(*options.Some); ok {
			if opts.ConnectTimeout != nil {
				cfg.connect = time.Duration(*opts.ConnectTimeout)
			}
			if opts.FirstByteTimeout != nil {
				cfg.firstByte = time.Duration(*opts.FirstByteTimeout)
			}
			if opts.BetweenBytesTimeout != nil {
				cfg.betweenBytes = time.Duration(*opts.BetweenBytesTimeout)
			}

			if client, ok := i.clientCache.Get(cfg); ok {
				return client
			}

			// 创建新的 Transport 和 Client
			transport := gohttp.DefaultTransport.(*gohttp.Transport).Clone()
			transport.DialContext = (&net.Dialer{
				Timeout:   cfg.connect,
				KeepAlive: 30 * time.Second,
			}).DialContext
			transport.ResponseHeaderTimeout = cfg.firstByte

			client := &gohttp.Client{
				Transport: transport,
				Timeout:   cfg.betweenBytes,
			}

			i.clientCache.Add(cfg, client)
			return client
		}
	}
	return gohttp.DefaultClient
}

// Handle 实现了 outgoing-handler.handle 接口。
// 这是执行 HTTP 请求的核心。
func (i *outgoingHandlerImpl) Handle(
	_ context.Context,
	request OutgoingRequest,
	options witgo.Option[RequestOptions], // options 是可选的
) witgo.Result[FutureIncomingResponse, ErrorCode] {
	// 1. 从管理器中获取我们之前构建的 OutgoingRequest 对象。
	req, ok := i.hm.OutgoingRequests.Get(request)
	if !ok {
		// 如果请求句柄无效，返回错误。
		return witgo.Err[FutureIncomingResponse, ErrorCode](ErrorCode{InternalError: witgo.SomePtr("invalid request handle")})
	}

	// 2. 将我们的内部 OutgoingRequest 结构转换为 Go 的标准 `http.Request`。
	goReq, err := i.buildGoRequest(req)
	if err != nil {
		return witgo.Err[FutureIncomingResponse, ErrorCode](ErrorCode{InternalError: witgo.SomePtr(err.Error())})
	}

	// 3. 创建一个 FutureIncomingResponse 资源。这是异步的关键。
	//    它包含一个 channel，后台的 goroutine 将通过它发送最终结果。
	future := &http.FutureIncomingResponse{
		ResultChan: make(chan http.Result, 1), // 缓冲为 1，以防发送时没有接收者
	}
	futureHandle := i.hm.Futures.Add(future)

	// 4. 启动一个新的 goroutine 来异步执行 HTTP 请求。
	go i.executeRequest(i.getClient(options), goReq, future)

	// 5. 立即返回 future 句柄，不阻塞。
	return witgo.Ok[FutureIncomingResponse, ErrorCode](futureHandle)
}

// executeRequest 在一个单独的 goroutine 中运行。
func (i *outgoingHandlerImpl) executeRequest(client *gohttp.Client, goReq *gohttp.Request, future *http.FutureIncomingResponse) {
	// 使用 Go 的默认 HTTP 客户端执行请求。
	resp, err := client.Do(goReq)

	// 将结果（成功或失败）发送到 future 的 channel 中。
	if err != nil {
		future.ResultChan <- http.Result{
			Err: err,
		}
		return
	}

	// 1. 转换 Headers
	respHeaders := make(http.Fields)
	for k, v := range resp.Header {
		respHeaders[strings.ToLower(k)] = v
	}
	headersHandle := i.hm.Fields.Add(respHeaders)

	// 2. 创建 FutureTrailers 资源
	futureTrailers := &http.FutureTrailers{
		ResultChan: make(chan http.ResultTrailers, 1),
	}
	futureTrailersID := i.hm.FutureTrailers.Add(futureTrailers)

	// 3. 创建一个包装了 trailers 逻辑的 reader，并为其创建 input-stream
	bodyReader := &trailerReader{
		body:         resp.Body,
		trailers:     resp.Trailer,
		trailersChan: futureTrailers.ResultChan,
		fieldsMgr:    i.hm.Fields,
	}
	streamID := i.hm.Streams.Add(&streams.Stream{Reader: bodyReader, Closer: bodyReader})

	// 4. 创建 IncomingBody，并链接 stream 和 future-trailers
	body := &http.IncomingBody{
		Body:         bodyReader,
		StreamHandle: streamID,
		Trailers:     futureTrailersID,
	}
	bodyID := i.hm.IncomingBodies.Add(body)

	// 5. 创建 IncomingResponse 资源, 并链接 body
	incomingResponse := &http.IncomingResponse{
		Response:   resp,
		Headers:    headersHandle,
		BodyHandle: bodyID, // 关键链接
	}
	responseHandle := i.hm.Responses.Add(incomingResponse)

	// 6. 将成功的响应句柄发送到 channel。
	future.ResultChan <- http.Result{
		ResponseHandle: responseHandle,
	}
}

// buildGoRequest 是一个辅助函数，用于将 wasi-http 请求转换为 Go 的 http.Request。
func (i *outgoingHandlerImpl) buildGoRequest(req *http.OutgoingRequest) (*gohttp.Request, error) {
	// 构造 URL
	scheme := "https"
	if req.Scheme != nil && *req.Scheme == "http" {
		scheme = "http"
	}
	if req.Authority == nil {
		return nil, fmt.Errorf("request authority cannot be empty")
	}
	url := fmt.Sprintf("%s://%s%s", scheme, *req.Authority, req.Path)

	// 创建 Go 的 http.Request。req.Body 是一个 io.PipeReader，
	// 当 Guest 向 outgoing-body 写入数据时，这里就能读到。
	goReq, err := gohttp.NewRequest(req.Method, url, req.Body)
	if err != nil {
		return nil, err
	}

	// 填充 Headers
	headers, ok := i.hm.Fields.Get(req.Headers)
	if !ok {
		return nil, fmt.Errorf("invalid fields handle for request headers")
	}
	for k, v := range headers {
		goReq.Header[gohttp.CanonicalHeaderKey(k)] = v
	}

	return goReq, nil
}

// trailerReader 包装了 http.Response.Body，用于在读取到 EOF 时，
// 将 trailers 发送到 channel 中。
type trailerReader struct {
	body         io.ReadCloser
	trailers     gohttp.Header
	trailersChan chan<- http.ResultTrailers
	fieldsMgr    *http.FieldsManager
	sent         bool
}

func (r *trailerReader) Read(p []byte) (n int, err error) {
	n, err = r.body.Read(p)
	if err == io.EOF && !r.sent {
		r.sendTrailers(nil)
	}
	return
}

func (r *trailerReader) Close() error {
	if !r.sent {
		r.sendTrailers(errors.New("body closed before trailers were received"))
	}
	return r.body.Close()
}

func (r *trailerReader) sendTrailers(err error) {
	r.sent = true
	var trailersID uint32
	if err == nil && len(r.trailers) > 0 {
		trailersID = r.fieldsMgr.Add(http.Fields(r.trailers))
	}
	r.trailersChan <- http.ResultTrailers{TrailersHandle: trailersID, Err: err}
	close(r.trailersChan)
}
