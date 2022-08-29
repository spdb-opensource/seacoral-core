package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ NetworkAPI = &clientConfig{}

type NetworkAPI interface {
	PostNetwork(ctx context.Context, config api.NetworkConfig) (api.TaskObjectResponse, error)
	DeleteNetwork(ctx context.Context, id string) error
	UpdateNetwork(ctx context.Context, id string, opts api.NetworkOptions) (api.TaskObjectResponse, error)
	ListNetworks(ctx context.Context, name, id, siteId, clusterId, topology string) ([]api.Network, error)
}

// NewNetworkAPI returns a NetworkAPI
func NewNetworkAPI(host string, cli client.Client) NetworkAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) PostNetwork(ctx context.Context, config api.NetworkConfig) (api.TaskObjectResponse, error) {
	const uri = "/v1.0/manager/networks"

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

func (c *clientConfig) DeleteNetwork(ctx context.Context, id string) error {
	uri := "/v1.0/manager/networks/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateNetwork(ctx context.Context, id string, opts api.NetworkOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/networks/" + id

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

func (c *clientConfig) ListNetworks(ctx context.Context, name, id, siteId, clusterId, topology string) ([]api.Network, error) {
	params := make(url.Values)

	if name != "" {
		params.Set("name", name)
	}

	if id != "" {
		params.Set("id", id)
	}

	if siteId != "" {
		params.Set("site_id", siteId)
	}

	if clusterId != "" {
		params.Set("cluster_id", clusterId)
	}

	url := url.URL{
		Path:     "/v1.0/manager/networks",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Network

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err

}
