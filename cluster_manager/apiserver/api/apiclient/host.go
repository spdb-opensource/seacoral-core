package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ HostAPI = &clientConfig{}

type HostAPI interface {
	PostHost(ctx context.Context, config api.HostConfig) (api.TaskObjectResponse, error)
	DeleteHost(ctx context.Context, id string) error
	UpdateHost(ctx context.Context, id string, opts api.HostOptions) (api.TaskObjectResponse, error)
	ListHosts(ctx context.Context, id, name, siteId, clusterId string) ([]api.Host, error)
}

func NewHostAPI(host string, cli client.Client) HostAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) PostHost(ctx context.Context, config api.HostConfig) (api.TaskObjectResponse, error) {
	const uri = "/v1.0/manager/hosts"

	resp, err := requireOK(c.client.Post(ctx, uri, config))
	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	defer resp.Body.Close()

	s := api.TaskObjectResponse{}

	err = decodeBody(resp, &s)
	if err != nil {
		return api.TaskObjectResponse{}, errors.Errorf("%s %s%s,%v", http.MethodPost, c.host, resp.Request.URL.String(), err)
	}

	return s, err
}

func (c *clientConfig) DeleteHost(ctx context.Context, id string) error {
	uri := "/v1.0/manager/hosts/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateHost(ctx context.Context, id string, opts api.HostOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/hosts/" + id

	resp, err := requireOK(c.client.Put(ctx, uri, opts))
	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	defer resp.Body.Close()

	s := api.TaskObjectResponse{}

	err = decodeBody(resp, &s)
	if err != nil {
		return api.TaskObjectResponse{}, errors.Errorf("%s %s%s,%v", http.MethodPut, c.host, resp.Request.URL.String(), err)
	}

	return s, err
}
func (c *clientConfig) ListHosts(ctx context.Context, id, name, siteId, clusterId string) ([]api.Host, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if name != "" {
		params.Set("name", name)
	}

	if siteId != "" {
		params.Set("site_id", siteId)
	}

	if clusterId != "" {
		params.Set("cluster_id", clusterId)
	}

	url := url.URL{
		Path:     "/v1.0/manager/hosts",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Host

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}
