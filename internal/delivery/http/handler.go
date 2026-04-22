package http

import (
	"io"
	stdhttp "net/http"

	"caching-proxy/internal/usecase"
)

type Proxy interface {
	Handle(req usecase.Request) (*usecase.Response, error)
}

type Handler struct {
	proxy Proxy
}

func NewHandler(proxy Proxy) *Handler {
	return &Handler{
		proxy: proxy,
	}
}

func (h *Handler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		stdhttp.Error(w, "read request body", stdhttp.StatusBadRequest)
		return
	}

	resp, err := h.proxy.Handle(usecase.Request{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Headers:  cloneHeaders(r.Header),
		Body:     body,
	})
	if err != nil {
		stdhttp.Error(w, "proxy request failed", stdhttp.StatusBadGateway)
		return
	}

	writeHeaders(w.Header(), resp.Headers)
	w.WriteHeader(resp.StatusCode)

	if _, err := w.Write(resp.Body); err != nil {
		return
	}
}

func writeHeaders(dst stdhttp.Header, src map[string][]string) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func cloneHeaders(headers stdhttp.Header) map[string][]string {
	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}
