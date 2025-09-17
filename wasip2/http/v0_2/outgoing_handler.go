package v0_2

import (
	"context"
	"fmt"
	"net"
	gohttp "net/http"
	"strings"
	"time"
	"wazero-wasip2/internal/http"
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

// executeRequest 在一个单独的 goroutine 中运行。
func (i *outgoingHandlerImpl) executeRequest(client *gohttp.Client, goReq *gohttp.Request, future *http.FutureIncomingResponse) {
	// 使用 Go 的默认 HTTP 客户端执行请求。
	resp, err := client.Do(goReq)

	// 将结果（成功或失败）发送到 future 的 channel 中。
	if err != nil {
		// 如果网络请求失败，我们创建一个 error 资源。
		// 这里的实现可以更精细，将 Go 的 net.Error 映射到具体的 wasi:http/types.error-code。
		// 为简化起见，我们统一使用 InternalError。
		future.ResultChan <- http.Result{
			Err: err,
		}
		return
	}

	// 请求成功，我们将 http.Response 转换为我们的 wasi-http 资源。

	// 1. 转换 Headers
	respHeaders := make(http.Fields)
	for k, v := range resp.Header {
		respHeaders[strings.ToLower(k)] = v
	}
	headersHandle := i.hm.Fields.Add(respHeaders)

	// 2. 创建 IncomingResponse 资源
	incomingResponse := &http.IncomingResponse{
		Response: resp,
		Headers:  headersHandle,
	}
	responseHandle := i.hm.Responses.Add(incomingResponse)

	// 3. 将成功的响应句柄发送到 channel。
	future.ResultChan <- http.Result{
		ResponseHandle: responseHandle,
	}
}
