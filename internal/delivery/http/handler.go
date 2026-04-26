package http

import (
	"errors"
	"io"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"

	"caching-proxy/internal/usecase"
)

// Proxy is the use-case contract the delivery layer depends on.
type Proxy interface {
	Handle(req usecase.ProxyRequest) (*usecase.ProxyResponse, error)
}

// NewStdHandler returns a stdlib http.Handler that proxies through the use case.
func NewStdHandler(proxy Proxy) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			stdhttp.Error(w, "read request body", stdhttp.StatusBadRequest)
			return
		}

		resp, err := proxy.Handle(buildProxyRequest(r, body))
		if err != nil {
			status, msg := errorToStatus(err)
			stdhttp.Error(w, msg, status)
			return
		}

		writeProxyResponse(w, resp)
	})
}

// NewGinEngine returns a gin engine that routes every request through the use case.
func NewGinEngine(proxy Proxy) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.NoRoute(func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(stdhttp.StatusBadRequest, "read request body")
			return
		}

		resp, err := proxy.Handle(buildProxyRequest(c.Request, body))
		if err != nil {
			status, msg := errorToStatus(err)
			c.String(status, msg)
			return
		}

		writeProxyResponse(c.Writer, resp)
	})
	return engine
}

func buildProxyRequest(r *stdhttp.Request, body []byte) usecase.ProxyRequest {
	return usecase.ProxyRequest{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Headers:  cloneHeaders(r.Header),
		Body:     body,
	}
}

func writeProxyResponse(w stdhttp.ResponseWriter, resp *usecase.ProxyResponse) {
	for k, vs := range resp.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func errorToStatus(err error) (int, string) {
	if errors.Is(err, usecase.ErrOrigin) {
		return stdhttp.StatusBadGateway, "origin request failed"
	}
	return stdhttp.StatusInternalServerError, "proxy error"
}

func cloneHeaders(headers stdhttp.Header) map[string][]string {
	cloned := make(map[string][]string, len(headers))
	for k, v := range headers {
		cp := make([]string, len(v))
		copy(cp, v)
		cloned[k] = cp
	}
	return cloned
}
