package v0_2

import (
	"net/http"
	"strings"
	witgo "wazero-wasip2/wit-go"
)

// fromWasiMethod 将 WIT 的 Method variant 转换为 Go 的 HTTP 方法字符串。
func fromWasiMethod(method Method) string {
	switch {
	case method.Get != nil:
		return http.MethodGet
	case method.Head != nil:
		return http.MethodHead
	case method.Post != nil:
		return http.MethodPost
	case method.Put != nil:
		return http.MethodPut
	case method.Delete != nil:
		return http.MethodDelete
	case method.Connect != nil:
		return http.MethodConnect
	case method.Options != nil:
		return http.MethodOptions
	case method.Trace != nil:
		return http.MethodTrace
	case method.Patch != nil:
		return http.MethodPatch
	case method.Other != nil:
		return *method.Other
	default:
		return "" // 或者返回一个错误
	}
}

// toWasiMethod 将 Go 的 HTTP 方法字符串转换为 WIT 的 Method variant。
func toWasiMethod(method string) Method {
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return Method{Get: &witgo.Unit{}}
	case http.MethodHead:
		return Method{Head: &witgo.Unit{}}
	case http.MethodPost:
		return Method{Post: &witgo.Unit{}}
	case http.MethodPut:
		return Method{Put: &witgo.Unit{}}
	case http.MethodDelete:
		return Method{Delete: &witgo.Unit{}}
	case http.MethodConnect:
		return Method{Connect: &witgo.Unit{}}
	case http.MethodOptions:
		return Method{Options: &witgo.Unit{}}
	case http.MethodTrace:
		return Method{Trace: &witgo.Unit{}}
	case http.MethodPatch:
		return Method{Patch: &witgo.Unit{}}
	default:
		// 对于不在标准列表中的方法，使用 Other case。
		return Method{Other: &method}
	}
}

// fromWasiScheme 将 WIT 的 Scheme variant 转换为 Go 的 URL scheme 字符串指针。
func fromWasiScheme(scheme Scheme) *string {
	var s string
	switch {
	case scheme.HTTP != nil:
		s = "http"
	case scheme.HTTPS != nil:
		s = "https"
	case scheme.Other != nil:
		s = *scheme.Other
	default:
		return nil
	}
	return &s
}

// toWasiScheme 将 Go 的 URL scheme 字符串转换为 WIT 的 Scheme variant。
func toWasiScheme(scheme string) Scheme {
	switch strings.ToLower(scheme) {
	case "http":
		return Scheme{HTTP: &witgo.Unit{}}
	case "https":
		return Scheme{HTTPS: &witgo.Unit{}}
	default:
		return Scheme{Other: &scheme}
	}
}
