package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ ClusterAPI = &clientConfig{}

type ClusterAPI interface {
	PostCluster(ctx context.Context, config api.ClusterConfig) (api.ObjectResponse, error)
	DeleteCluster(ctx context.Context, id string) error
	UpdateCluster(ctx context.Context, id string, opts api.ClusterOptions) (api.ObjectResponse, error)
	ListClusters(ctx context.Context, id, name, siteId string) ([]api.Cluster, error)
}

// NewClusterAPI returns a ClusterAPI
func NewClusterAPI(host string, cli client.Client) ClusterAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) PostCluster(ctx context.Context, config api.ClusterConfig) (api.ObjectResponse, error) {
	const uri = "/v1.0/manager/clusters"

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

func (c *clientConfig) DeleteCluster(ctx context.Context, id string) error {
	uri := "/v1.0/manager/clusters/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateCluster(ctx context.Context, id string, opts api.ClusterOptions) (api.ObjectResponse, error) {
	uri := "/v1.0/manager/clusters/" + id

	resp, err := requireOK(c.client.Put(ctx, uri, opts))
	if err != nil {
		return api.ObjectResponse{}, err
	}
	defer resp.Body.Close()

	s := api.ObjectResponse{}

	err = decodeBody(resp, &s)
	if err != nil {
		return api.ObjectResponse{}, errors.Errorf("%s %s%s,%v", http.MethodPut, c.host, resp.Request.URL.String(), err)
	}

	return s, err
}

func (c *clientConfig) ListClusters(ctx context.Context, id, name, siteId string) ([]api.Cluster, error) {
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

	url := url.URL{
		Path:     "/v1.0/manager/clusters",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Cluster

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err

}
