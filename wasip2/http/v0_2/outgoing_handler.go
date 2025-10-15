package v0_2

import (
	"fmt"
	"net"
	gohttp "net/http"
	"time"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	lru "github.com/hashicorp/golang-lru/v2"
)

type timeoutConfig struct {
	connect      time.Duration
	firstByte    time.Duration
	betweenBytes time.Duration
}

// outgoingHandlerImpl 封装了 wasi:http/outgoing-handler 的所有操作。
type outgoingHandlerImpl struct {
	hm          *manager_http.HTTPManager
	clientCache *lru.Cache[timeoutConfig, *gohttp.Client]
}

func newOutgoingHandlerImpl(hm *manager_http.HTTPManager) *outgoingHandlerImpl {
	clientCache, _ := lru.New[timeoutConfig, *gohttp.Client](32)
	return &outgoingHandlerImpl{hm, clientCache}
}

func (i *outgoingHandlerImpl) getClient(opts *manager_http.RequestOptions) *gohttp.Client {
	// 1. 确定超时配置。如果无特定选项，则使用零值配置。
	var cfg timeoutConfig
	if opts != nil {
		if opts.ConnectTimeout != nil {
			cfg.connect = time.Duration(*opts.ConnectTimeout)
		}
		if opts.FirstByteTimeout != nil {
			cfg.firstByte = time.Duration(*opts.FirstByteTimeout)
		}
		if opts.BetweenBytesTimeout != nil {
			cfg.betweenBytes = time.Duration(*opts.BetweenBytesTimeout)
		}
	}

	// 2. 使用配置作为键，检查缓存。
	//    零值的 cfg 将作为我们自定义的 "默认客户端" 的键。
	if client, ok := i.clientCache.Get(cfg); ok {
		return client
	}

	// 3. 如果缓存未命中，则创建一个新的客户端。
	//    这块逻辑现在同时服务于自定义客户端和默认客户端。
	transport := gohttp.DefaultTransport.(*gohttp.Transport).Clone()

	// 保留原始代码中的代理和 TLS 设置
	// p, _ := url.Parse("http://192.168.3.121:8888")
	// transport.Proxy = gohttp.ProxyURL(p)
	transport.Proxy = gohttp.ProxyFromEnvironment
	transport.TLSClientConfig.InsecureSkipVerify = true

	// 根据配置设置超时（如果cfg为零值，则超时为0，表示不设限制）
	transport.DialContext = (&net.Dialer{
		Timeout:   cfg.connect,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.ResponseHeaderTimeout = cfg.firstByte

	// 重定向和CookieJar由guest接管
	client := &gohttp.Client{
		Transport: transport,
		Timeout:   cfg.betweenBytes, // 整个请求的超时，包括读取响应体
		Jar:       nil,
		CheckRedirect: func(req *gohttp.Request, via []*gohttp.Request) error {
			return gohttp.ErrUseLastResponse
		},
	}

	// 4. 将新创建的客户端存入缓存并返回
	i.clientCache.Add(cfg, client)
	return client
}

// Handle 实现了 outgoing-handler.handle 接口。
// 这是执行 HTTP 请求的核心。
// 调用后会消耗掉request和options
func (i *outgoingHandlerImpl) Handle(
	request OutgoingRequest,
	options witgo.Option[RequestOptions], // options 是可选的
) witgo.Result[FutureIncomingResponse, ErrorCode] {
	// 1. 从管理器中获取我们之前构建的 OutgoingRequest 对象。
	// 消耗掉资源
	var opts *manager_http.RequestOptions
	if options.IsSome() {
		opts, _ = i.hm.Options.Pop(*options.Some)
	}

	req, ok := i.hm.OutgoingRequests.Pop(request)
	if !ok {
		// 如果请求句柄无效，返回错误。
		return witgo.Err[FutureIncomingResponse, ErrorCode](ErrorCode{InternalError: witgo.SomePtr("invalid request handle")})
	}

	// 2. 将我们的内部 OutgoingRequest 结构转换为 Go 的标准 `http.Request`。
	// req 的 Close 转移给 goReq 控制
	goReq, err := i.buildGoRequest(req)
	if err != nil {
		return witgo.Err[FutureIncomingResponse, ErrorCode](ErrorCode{InternalError: witgo.SomePtr(err.Error())})
	}

	client := i.getClient(opts)

	// 3. 创建一个 FutureIncomingResponse 资源。这是异步的关键。
	//    它包含一个 channel，后台的 goroutine 将通过它发送最终结果。
	future := &manager_http.FutureIncomingResponse{
		Pollable: manager_io.NewPollable(nil),
	}

	// 4. 启动一个新的 goroutine 来异步执行 HTTP 请求。
	go i.executeRequest(client, goReq, future)

	// 5. 立即返回 future 句柄，不阻塞。
	futureHandle := i.hm.Futures.Add(future)
	return witgo.Ok[FutureIncomingResponse, ErrorCode](futureHandle)
}

// executeRequest 在一个单独的 goroutine 中运行。
func (i *outgoingHandlerImpl) executeRequest(client *gohttp.Client, goReq *gohttp.Request, future *manager_http.FutureIncomingResponse) {
	defer func() {
		future.Pollable.SetReady()
	}()

	resp, err := client.Do(goReq)

	if err != nil {
		future.Result = manager_http.Result{
			Err: err,
		}
		return
	}
	future.Result = manager_http.Result{
		Response: resp,
	}
}

// buildGoRequest 是一个辅助函数，用于将 wasi-http 请求转换为 Go 的 http.Request。
func (i *outgoingHandlerImpl) buildGoRequest(req *manager_http.OutgoingRequest) (*gohttp.Request, error) {
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

	// 这里如果直接赋值会导致小写User-Agent和大小共存
	for k, vv := range req.Headers {
		for _, v := range vv {
			goReq.Header.Add(k, v)
		}
	}
	goReq.Trailer = make(gohttp.Header)

	req.Request = goReq
	return goReq, nil
}
