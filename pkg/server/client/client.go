package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context/ctxhttp"
)

const defaultTimeout = 2 * time.Minute

type encoder struct {
	bodyType string
	encode   func(obj interface{}) (io.Reader, error)
}

type client struct {
	timeout time.Duration
	scheme  string
	address string
	client  *http.Client
	enc     *encoder
}

// Client is an HTTP client
type Client interface {
	Do(ctx context.Context, method, url string, obj interface{}) (*http.Response, error)

	Get(ctx context.Context, url string) (*http.Response, error)

	Head(ctx context.Context, url string) (*http.Response, error)

	Post(ctx context.Context, url string, body interface{}) (*http.Response, error)

	Put(ctx context.Context, url string, body interface{}) (*http.Response, error)

	Delete(ctx context.Context, url string) (*http.Response, error)
}

// NewClient returns Client,implements by *client
func NewClient(addr string, timeout time.Duration, tlsConfig *tls.Config) Client {

	if timeout == 0 {
		timeout = defaultTimeout
	}

	trans := defaultPooledTransport(timeout)

	if parts := strings.SplitN(addr, "unix://", 2); len(parts) == 2 {
		trans.Dial = func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", parts[1])
		}

		addr = parts[1]
	}

	scheme := "http"

	if tlsConfig != nil {
		trans.TLSClientConfig = tlsConfig
		scheme = "https"
	}

	enc := &encoder{
		bodyType: "application/json",
		encode:   encodeJSON,
	}

	c := &client{
		timeout: timeout,
		scheme:  scheme,
		address: addr,
		enc:     enc,
		client: &http.Client{
			Transport: trans,
			Timeout:   timeout,
		},
	}

	return c
}

func (c *client) SetEncoder(bodyType string, enc func(obj interface{}) (io.Reader, error)) {
	c.enc = &encoder{
		bodyType: bodyType,
		encode:   enc,
	}
}

func (c *client) Do(ctx context.Context, method, url string, obj interface{}) (*http.Response, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
	}

	req, err := c.newRequest(method, url, obj)
	if err != nil {
		return nil, err
	}

	resp, err := ctxhttp.Do(ctx, c.client, req)
	if err == nil {
		return resp, nil
	}

	return resp, errors.Wrap(err, req.RequestURI)
}

func (c *client) Get(ctx context.Context, url string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil)
}

func (c *client) Head(ctx context.Context, url string) (*http.Response, error) {
	return c.Do(ctx, http.MethodHead, url, nil)
}

func (c *client) Post(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, url, body)
}

func (c *client) Put(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, url, body)
}

func (c *client) postForm(ctx context.Context, uri string, data url.Values) (*http.Response, error) {
	enc := c.enc

	if data != nil {
		c.enc = &encoder{
			bodyType: "application/x-www-form-urlencoded",
			encode: func(obj interface{}) (io.Reader, error) {
				data, ok := obj.(url.Values)
				if !ok {
					return nil, errors.New("wanted obj type error")
				}
				return strings.NewReader(data.Encode()), nil
			},
		}
	}

	resp, err := c.Post(ctx, uri, data)
	c.enc = enc

	return resp, err
}

func (c *client) Delete(ctx context.Context, url string) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, url, nil)
}

// uri encoded by url.URL.RequestURI()
func (c *client) newRequest(method, url string, obj interface{}) (*http.Request, error) {
	var (
		err  error
		body io.Reader
	)
	if obj != nil && c.enc != nil {
		body, err = c.enc.encode(obj)
		if err != nil {
			return nil, errors.Errorf("%s:%s/%s,%s", method, c.address, url, err)
		}
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, errors.Errorf("%s:%s/%s,%s", method, c.address, url, err)
	}

	req.URL.Host = c.address
	req.URL.Scheme = c.scheme
	req.Host = c.address
	if body != nil {
		req.Header.Set("Content-Type", c.enc.bodyType)
	}

	return req, nil
}

// RequireOK is used to wrap doRequest and check for a 200
func RequireOK(resp *http.Response, e error) (*http.Response, error) {
	if e != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, e
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		buf := bytes.NewBuffer(nil)

		io.Copy(buf, resp.Body)
		resp.Body.Close()

		return nil, errors.Errorf("%s,Unexpected response code: %d (%s)", resp.Request.URL.String(), resp.StatusCode, buf.Bytes())
	}

	return resp, nil
}

// EnsureBodyClose close *http.Response
func EnsureBodyClose(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.CopyN(ioutil.Discard, resp.Body, 512)

		resp.Body.Close()
	}
}

// defaultTransport returns a new http.Transport with the same default values
// as http.DefaultTransport, but with idle connections and keepalives disabled.
func defaultTransport(timeout time.Duration) *http.Transport {
	transport := defaultPooledTransport(timeout)
	transport.DisableKeepAlives = true
	transport.MaxIdleConnsPerHost = -1
	return transport
}

// defaultPooledTransport returns a new http.Transport with similar default
// values to http.DefaultTransport. Do not use this for transient transports as
// it can leak file descriptors over time. Only use this for transports that
// will be re-used for the same host(s).
func defaultPooledTransport(timeout time.Duration) *http.Transport {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: timeout * 2,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 1,
	}
	return transport
}

// defaultClient returns a new http.Client with similar default values to
// http.Client, but with a non-shared Transport, idle connections disabled, and
// keepalives disabled.
func defaultClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: defaultTransport(timeout),
	}
}

// defaultPooledClient returns a new http.Client with the same default values
// as http.Client, but with a shared Transport. Do not use this function
// for transient clients as it can leak file descriptors over time. Only use
// this for clients that will be re-used for the same host(s).
func defaultPooledClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: defaultPooledTransport(timeout),
	}
}

// decodeBody is used to JSON decode a body
func decodeBody(resp *http.Response, out interface{}) error {
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

// encodeJSON is used to encode a request body
func encodeJSON(obj interface{}) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)

	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		return nil, err
	}

	return buf, nil
}
