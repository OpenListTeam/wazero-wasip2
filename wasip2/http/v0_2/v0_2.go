package v0_2

import (
	"context"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	"github.com/foxxorcat/wazero-wasip2/wasip2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:http/types@0.2.0 implementation ---

type httpTypes struct {
	hm *manager_http.HTTPManager
}

func NewTypes(hm *manager_http.HTTPManager) wasip2.Implementation {
	return &httpTypes{hm: hm}
}

func (i *httpTypes) Name() string { return "wasi:http/types" }
func (i *httpTypes) Versions() []string {
	return []string{"0.2", "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *httpTypes) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	exporter := witgo.NewExporter(builder)

	hm := h.HTTPManager()
	// --- fields ---
	fieldsHandler := newFieldsImpl(hm.Fields)
	exporter.Export("[constructor]fields", fieldsHandler.Constructor)
	exporter.Export("[static]fields.from-list", fieldsHandler.FromList)
	exporter.Export("[resource-drop]fields", fieldsHandler.Drop)
	exporter.Export("[method]fields.get", fieldsHandler.Get)
	exporter.Export("[method]fields.has", fieldsHandler.Has)
	exporter.Export("[method]fields.set", fieldsHandler.Set)
	exporter.Export("[method]fields.delete", fieldsHandler.Delete)
	exporter.Export("[method]fields.append", fieldsHandler.Append)
	exporter.Export("[method]fields.entries", fieldsHandler.Entries)
	exporter.Export("[method]fields.clone", fieldsHandler.Clone)

	// --- incoming-request ---
	incomingRequestHandler := newIncomingRequestImpl(hm)
	exporter.Export("[resource-drop]incoming-request", incomingRequestHandler.Drop)
	exporter.Export("[method]incoming-request.method", incomingRequestHandler.Method)
	exporter.Export("[method]incoming-request.path-with-query", incomingRequestHandler.PathWithQuery)
	exporter.Export("[method]incoming-request.scheme", incomingRequestHandler.Scheme)
	exporter.Export("[method]incoming-request.authority", incomingRequestHandler.Authority)
	exporter.Export("[method]incoming-request.headers", incomingRequestHandler.Headers)
	exporter.Export("[method]incoming-request.consume", incomingRequestHandler.Consume)

	// --- outgoing-request ---
	outgoingRequestHandler := newOutgoingRequestImpl(hm)
	exporter.Export("[constructor]outgoing-request", outgoingRequestHandler.Constructor)
	exporter.Export("[resource-drop]outgoing-request", outgoingRequestHandler.Drop)
	exporter.Export("[method]outgoing-request.body", outgoingRequestHandler.Body)
	exporter.Export("[method]outgoing-request.method", outgoingRequestHandler.Method)
	exporter.Export("[method]outgoing-request.set-method", outgoingRequestHandler.SetMethod)
	exporter.Export("[method]outgoing-request.path-with-query", outgoingRequestHandler.PathWithQuery)
	exporter.Export("[method]outgoing-request.set-path-with-query", outgoingRequestHandler.SetPathWithQuery)
	exporter.Export("[method]outgoing-request.scheme", outgoingRequestHandler.Scheme)
	exporter.Export("[method]outgoing-request.set-scheme", outgoingRequestHandler.SetScheme)
	exporter.Export("[method]outgoing-request.authority", outgoingRequestHandler.Authority)
	exporter.Export("[method]outgoing-request.set-authority", outgoingRequestHandler.SetAuthority)
	exporter.Export("[method]outgoing-request.headers", outgoingRequestHandler.Headers)

	// --- request-options ---
	requestOptionsHandler := newRequestOptionsImpl(hm)
	exporter.Export("[constructor]request-options", requestOptionsHandler.Constructor)
	exporter.Export("[resource-drop]request-options", requestOptionsHandler.Drop)
	exporter.Export("[method]request-options.connect-timeout", requestOptionsHandler.ConnectTimeout)
	exporter.Export("[method]request-options.set-connect-timeout", requestOptionsHandler.SetConnectTimeout)
	exporter.Export("[method]request-options.first-byte-timeout", requestOptionsHandler.FirstByteTimeout)
	exporter.Export("[method]request-options.set-first-byte-timeout", requestOptionsHandler.SetFirstByteTimeout)
	exporter.Export("[method]request-options.between-bytes-timeout", requestOptionsHandler.BetweenBytesTimeout)
	exporter.Export("[method]request-options.set-between-bytes-timeout", requestOptionsHandler.SetBetweenBytesTimeout)

	// --- response-outparam ---
	responseOutparamHandler := newResponseOutparamImpl(hm)
	exporter.Export("[resource-drop]response-outparam", responseOutparamHandler.Drop)
	exporter.Export("[static]response-outparam.set", responseOutparamHandler.Set)

	// --- incoming-response ---
	responseHandler := newIncomingResponseImpl(hm)
	exporter.Export("[resource-drop]incoming-response", responseHandler.Drop)
	exporter.Export("[method]incoming-response.status", responseHandler.Status)
	exporter.Export("[method]incoming-response.headers", responseHandler.Headers)
	exporter.Export("[method]incoming-response.consume", responseHandler.Consume)

	// --- incoming-body ---
	bodyHandler := newIncomingBodyImpl(hm)
	exporter.Export("[resource-drop]incoming-body", bodyHandler.Drop)
	exporter.Export("[method]incoming-body.stream", bodyHandler.Stream)
	exporter.Export("[static]incoming-body.finish", bodyHandler.Finish)

	// --- future-trailers ---
	futureTrailersHandler := newFutureTrailersImpl(hm)
	exporter.Export("[resource-drop]future-trailers", futureTrailersHandler.Drop)
	exporter.Export("[method]future-trailers.subscribe", futureTrailersHandler.Subscribe)
	exporter.Export("[method]future-trailers.get", futureTrailersHandler.Get)

	// --- outgoing-response ---
	outgoingResponseHandler := newOutgoingResponseImpl(hm)
	exporter.Export("[constructor]outgoing-response", outgoingResponseHandler.Constructor)
	exporter.Export("[resource-drop]outgoing-response", outgoingResponseHandler.Drop)
	exporter.Export("[method]outgoing-response.status-code", outgoingResponseHandler.StatusCode)
	exporter.Export("[method]outgoing-response.set-status-code", outgoingResponseHandler.SetStatusCode)
	exporter.Export("[method]outgoing-response.headers", outgoingResponseHandler.Headers)
	exporter.Export("[method]outgoing-response.body", outgoingResponseHandler.Body)

	// --- outgoing-body ---
	outgoingBodyHandler := newOutgoingBodyImpl(hm)
	exporter.Export("[resource-drop]outgoing-body", outgoingBodyHandler.Drop)
	exporter.Export("[method]outgoing-body.write", outgoingBodyHandler.Write)
	exporter.Export("[static]outgoing-body.finish", outgoingBodyHandler.Finish)

	// --- future-incoming-response ---
	futureHandler := newFutureIncomingResponseImpl(hm)
	exporter.Export("[resource-drop]future-incoming-response", futureHandler.Drop)
	exporter.Export("[method]future-incoming-response.subscribe", futureHandler.Subscribe)
	exporter.Export("[method]future-incoming-response.get", futureHandler.Get)

	// 导出核心的 handle 函数。
	handler := newOutgoingHandlerImpl(h.HTTPManager())
	exporter.Export("handle", handler.Handle)

	// --- incoming-handler (placeholder for world linking) ---
	// Although the guest exports this, we define it here so that the host
	// knows about the interface when linking a world that uses it.
	incomingHandler := NewIncomingHandler(hm)
	// We don't export any functions for it from the host side.
	_ = incomingHandler

	// --- http-error-code ---
	exporter.Export("http-error-code", func(err WasiError) witgo.Option[ErrorCode] {
		// Get the underlying Go error from the wasi:io/error resource handle
		goErr, ok := h.ErrorManager().Get(err)
		if !ok {
			return witgo.None[ErrorCode]()
		}

		// Map the Go error to a wasi:http ErrorCode
		httpErr := mapGoErrToWasiHttpErr(goErr)

		// Check if it's an unclassified "internal error"
		if httpErr.InternalError != nil && httpErr.InternalError.Some != nil && *httpErr.InternalError.Some == goErr.Error() {
			return witgo.None[ErrorCode]()
		}

		return witgo.Some(httpErr)
	})
	return nil
}

// --- wasi:http/outgoing-handler@0.2.0 implementation ---

type outgoingHandler struct {
	hm *manager_http.HTTPManager
}

func NewOutgoingHandler(hm *manager_http.HTTPManager) wasip2.Implementation {
	return &outgoingHandler{hm: hm}
}

func (i *outgoingHandler) Name() string { return "wasi:http/outgoing-handler" }
func (i *outgoingHandler) Versions() []string {
	return []string{"0.2", "0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *outgoingHandler) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	handler := newOutgoingHandlerImpl(h.HTTPManager())
	exporter := witgo.NewExporter(builder)
	exporter.Export("handle", handler.Handle)
	return nil
}
