package wasi_http

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	manager_http "github.com/OpenListTeam/wazero-wasip2/manager/http"
	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	v0_2 "github.com/OpenListTeam/wazero-wasip2/wasip2/http/v0_2"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero/api"
)

// Server 实现了 http.Handler，将传入的 HTTP 请求转发给 wasi-http guest 模块处理。
type Server struct {
	guest      api.Module
	wasiHost   *wasip2.Host
	witHost    *witgo.Host
	handleFunc api.Function
}

// NewServer 创建一个新的 wasi-http 服务器实例。
// guest 模块必须导出一个 `wasi:http/incoming-handler.handle` 函数。
func NewServer(guest api.Module, wasiHost *wasip2.Host) (*Server, error) {
	handleFunc := guest.ExportedFunction("wasi:http/incoming-handler#handle")
	if handleFunc == nil {
		return nil, fmt.Errorf("guest module must export wasi:http/incoming-handler#handle function")
	}

	witHost, err := witgo.NewHost(guest)
	if err != nil {
		return nil, fmt.Errorf("failed to create wit-go host: %w", err)
	}

	return &Server{
		guest:      guest,
		wasiHost:   wasiHost,
		witHost:    witHost,
		handleFunc: handleFunc,
	}, nil
}

// ServeHTTP 是 http.Handler 接口的实现。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hm := s.wasiHost.HTTPManager()

	// 1. 将 http.Request 转换为 wasi:http/types.incoming-request
	reqHandle, err := s.createIncomingRequest(ctx, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create incoming-request: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. 创建一个 response-outparam
	respChan := make(chan any, 1)
	outparam := &manager_http.ResponseOutparam{ResultChan: respChan}
	outparamHandle := hm.ResponseOutparams.Add(outparam)
	defer hm.ResponseOutparams.Remove(outparamHandle)

	// 3. 调用 guest 的 handle 函数
	_, err = s.handleFunc.Call(ctx, uint64(reqHandle), uint64(outparamHandle))
	if err != nil {
		// Guest 模块执行出错 (trap)
		http.Error(w, fmt.Sprintf("guest handle function trapped: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. 等待 guest 通过 response-outparam.set 返回结果
	select {
	case <-ctx.Done():
		http.Error(w, "context cancelled", http.StatusServiceUnavailable)
	case respResult := <-respChan:
		switch result := respResult.(type) {
		case v0_2.OutgoingResponse:
			s.writeOutgoingResponse(w, result)
		case v0_2.ErrorCode:
			// Guest 返回了一个错误
			http.Error(w, fmt.Sprintf("guest returned an error code: %+v", result), http.StatusInternalServerError)
		default:
			http.Error(w, "internal error: unknown type from response channel", http.StatusInternalServerError)
		}
	}
}

// createIncomingRequest 将 http.Request 转换为 wasi:http/types.incoming-request 资源
func (s *Server) createIncomingRequest(ctx context.Context, r *http.Request) (v0_2.IncomingRequest, error) {
	hm := s.wasiHost.HTTPManager()

	// 创建 Headers
	headers := make(manager_http.Fields)
	for k, v := range r.Header {
		headers[strings.ToLower(k)] = v
	}
	headersHandle := hm.Fields.Add(headers)

	// 构造最终的 IncomingRequest
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	req := &manager_http.IncomingRequest{
		Request: r,

		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Scheme:    &scheme,
		Authority: &r.Host,
		Headers:   headersHandle,

		Body: r.Body,
	}
	reqHandle := hm.IncomingRequests.Add(req)
	return reqHandle, nil
}

// writeOutgoingResponse 将 guest 返回的 OutgoingResponse 写入 http.ResponseWriter
func (s *Server) writeOutgoingResponse(w http.ResponseWriter, respHandle v0_2.OutgoingResponse) {
	hm := s.wasiHost.HTTPManager()
	resp, ok := hm.OutgoingResponses.Pop(respHandle)
	if !ok {
		http.Error(w, "internal error: invalid outgoing-response handle", http.StatusInternalServerError)
		return
	}
	resp.Response = w

	maps.Copy(w.Header(), resp.Headers)

	// 写入 Status Code
	w.WriteHeader(resp.StatusCode)

	// 写入 Body
	if resp.Body != nil {
		io.Copy(w, resp.Body)
	}
}
