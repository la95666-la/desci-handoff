package client

import (
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// HTTPClient 封装 fasthttp 客户端
type HTTPClient struct {
	client *fasthttp.Client
	once   sync.Once
}

var client HTTPClient

type Opt func(*fasthttp.Request)

func GetHTTPClient() *HTTPClient {
	client.once.Do(func() {
		client.client = &fasthttp.Client{
			MaxConnsPerHost: 100,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
		}
	})
	return &client
}

func (c *HTTPClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	err := c.client.Do(req, resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) Get(url string, opts ...Opt) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(req)
		}
	}
	var err error
	req.SetRequestURI(url)
	err = c.client.Do(req, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *HTTPClient) Post(url string, body []byte, opts ...Opt) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()

	req.SetBody(body)
	req.Header.SetMethod("POST")
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(req)
		}
	}
	var err error
	req.SetRequestURI(url)
	err = c.client.Do(req, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Clear 清空resp
func (c *HTTPClient) Clear(resp *fasthttp.Response) {
	if resp == nil {
		return
	}
	fasthttp.ReleaseResponse(resp)
}

func SetContentType(Type string) Opt {
	return func(req *fasthttp.Request) {
		req.Header.Set("Content-Type", Type)
	}
}

func SetJsonType() Opt {
	return func(req *fasthttp.Request) {
		req.Header.Set("Content-Type", "application/json")
	}
}
