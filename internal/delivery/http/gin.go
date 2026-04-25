package http

import (
	"io"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"

	"caching-proxy/internal/usecase"
)

func NewGinEngine(proxy Proxy) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.NoRoute(ginProxyHandler(proxy))
	return engine
}

func ginProxyHandler(proxy Proxy) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(stdhttp.StatusBadRequest, "read request body")
			return
		}

		resp, err := proxy.Handle(usecase.Request{
			Method:   c.Request.Method,
			Path:     c.Request.URL.Path,
			RawQuery: c.Request.URL.RawQuery,
			Headers:  cloneHeaders(c.Request.Header),
			Body:     body,
		})
		if err != nil {
			c.String(stdhttp.StatusBadGateway, "proxy request failed")
			return
		}

		writeHeaders(c.Writer.Header(), resp.Headers)
		c.Writer.WriteHeader(resp.StatusCode)
		_, _ = c.Writer.Write(resp.Body)
	}
}
