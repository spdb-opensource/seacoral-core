package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ SiteAPI = &clientConfig{}

type SiteAPI interface {
	AddSite(ctx context.Context, config api.SiteConfig) (api.ObjectResponse, error)
	DeleteSite(ctx context.Context, id string) error
	UpdateSite(ctx context.Context, id string, opts api.SiteOptions) error
	ListSites(ctx context.Context, id, name string) ([]api.Site, error)
}

type clientConfig struct {
	host   string
	client client.Client
}

// NewSiteAPI returns a SiteAPI
func NewSiteAPI(host string, cli client.Client) SiteAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) AddSite(ctx context.Context, config api.SiteConfig) (api.ObjectResponse, error) {
	const uri = "/v1.0/manager/sites"

	resp, err := requireOK(c.client.Post(ctx, uri, config))
	if err != nil {
		return api.ObjectResponse{}, err
	}
	defer resp.Body.Close()

	s := api.ObjectResponse{}

	err = decodeBody(resp, &s)
	if err != nil {
		return api.ObjectResponse{}, errors.Errorf("%s %s%s,%v", http.MethodPost, c.host, resp.Request.URL.String(), err)
	}

	return s, err
}

func (c *clientConfig) DeleteSite(ctx context.Context, id string) error {
	uri := "/v1.0/manager/sites/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateSite(ctx context.Context, id string, opts api.SiteOptions) error {
	uri := "/v1.0/manager/sites/" + id

	resp, err := requireOK(c.client.Put(ctx, uri, opts))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return err
}

func (c *clientConfig) ListSites(ctx context.Context, id, name string) ([]api.Site, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if name != "" {
		params.Set("name", name)
	}

	url := url.URL{
		Path:     "/v1.0/manager/sites",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Site

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}

// decodeBody is used to JSON decode a body
func decodeBody(resp *http.Response, out interface{}) error {
	dec := json.NewDecoder(resp.Body)

	return dec.Decode(out)
}

// requireOK is used to wrap doRequest and check for a 200
func requireOK(resp *http.Response, e error) (*http.Response, error) {
	if e != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return nil, e
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {

		buf := bytes.NewBuffer(nil)

		io.Copy(buf, resp.Body)
		resp.Body.Close()

		bodyStr := buf.String()

		out := api.ErrorResponse{}

		if err := json.NewDecoder(buf).Decode(&out); err != nil {
			return nil, errors.Errorf("%s,Unexpected response code: %d (%s,%s)", resp.Request.URL.String(), resp.StatusCode, bodyStr, err)
		}

		return nil, errors.Errorf("%s,Unexpected response code: %d (%s)", resp.Request.URL.String(), resp.StatusCode, out.Error)
	}

	return resp, nil
}
