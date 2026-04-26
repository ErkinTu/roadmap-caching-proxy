package origin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"caching-proxy/internal/usecase"
)

var ErrInvalidOrigin = errors.New("invalid origin URL")

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(rawBaseURL string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOrigin, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("%w: must include scheme and host", ErrInvalidOrigin)
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *Client) Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*usecase.OriginResponse, error) {
	target := c.buildURL(path, rawQuery)

	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build origin request: %w", err)
	}
	req.Header = cloneHeaders(headers)
	req.Host = c.baseURL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call origin: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read origin body: %w", err)
	}

	return &usecase.OriginResponse{
		StatusCode: resp.StatusCode,
		Headers:    cloneHeaders(resp.Header),
		Body:       respBody,
	}, nil
}

func (c *Client) buildURL(path, rawQuery string) string {
	target := *c.baseURL
	target.Path = joinPath(c.baseURL.Path, path)
	target.RawQuery = rawQuery
	return target.String()
}

func joinPath(basePath, requestPath string) string {
	if basePath == "" || basePath == "/" {
		if requestPath == "" {
			return "/"
		}
		return requestPath
	}
	if requestPath == "" || requestPath == "/" {
		return basePath
	}
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(requestPath, "/")
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string][]string, len(headers))
	for k, v := range headers {
		cp := make([]string, len(v))
		copy(cp, v)
		cloned[k] = cp
	}
	return cloned
}
