package origin

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"caching-proxy/internal/usecase"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(rawBaseURL string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("origin must include scheme and host")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*usecase.OriginResponse, error) {
	targetURL := c.buildURL(path, rawQuery)

	req, err := http.NewRequest(method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = cloneHeaders(headers)
	req.Host = c.baseURL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
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
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}
