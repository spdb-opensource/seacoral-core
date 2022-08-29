package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ RemoteAPI = &clientConfig{}

type RemoteAPI interface {
	PostRemoteStorage(ctx context.Context, config api.RemoteStorageConfig) (api.TaskObjectResponse, error)
	DeleteRemoteStorage(ctx context.Context, id string) error
	UpdateRemoteStorage(ctx context.Context, id string, opts api.RemoteStorageOptions) (api.TaskObjectResponse, error)
	ListRemoteStorages(ctx context.Context, id, name, siteId string) ([]api.RemoteStorage, error)

	//remotestoragepool
	PostRemoteStoragePool(ctx context.Context, storage string, config api.RemoteStoragePoolConfig) (api.TaskObjectResponse, error)
	DeleteRemoteStoragePool(ctx context.Context, storage, pool string) error
	UpdateRemoteStoragePool(ctx context.Context, storage, pool string, opts api.RemoteStoragePoolOptions) (api.ObjectResponse, error)
	ListRemoteStoragePools(ctx context.Context, id, name, storage string) ([]api.RemoteStoragePool, error)
}

func NewRemoteAPI(host string, cli client.Client) RemoteAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) PostRemoteStorage(ctx context.Context, config api.RemoteStorageConfig) (api.TaskObjectResponse, error) {
	const uri = "/v1.0/manager/storages/remote"

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

func (c *clientConfig) DeleteRemoteStorage(ctx context.Context, id string) error {
	uri := "/v1.0/manager/storages/remote/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateRemoteStorage(ctx context.Context, id string, opts api.RemoteStorageOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/storages/remote/" + id

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

func (c *clientConfig) ListRemoteStorages(ctx context.Context, id, name, siteId string) ([]api.RemoteStorage, error) {
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
		Path:     "/v1.0/manager/storages/remote",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.RemoteStorage

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}

func (c *clientConfig) PostRemoteStoragePool(ctx context.Context, storage string, config api.RemoteStoragePoolConfig) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/storages/remote/" + storage + "/pools"

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

func (c *clientConfig) DeleteRemoteStoragePool(ctx context.Context, storage, pool string) error {
	uri := "/v1.0/manager/storages/remote/" + storage + "/pools/" + pool

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateRemoteStoragePool(ctx context.Context, storage, pool string, opts api.RemoteStoragePoolOptions) (api.ObjectResponse, error) {
	uri := "/v1.0/manager/storages/remote/" + storage + "/pools/" + pool

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

func (c *clientConfig) ListRemoteStoragePools(ctx context.Context, id, name, storage string) ([]api.RemoteStoragePool, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if name != "" {
		params.Set("name", name)
	}

	url := url.URL{
		Path:     "/v1.0/manager/storages/remote/" + storage + "/pools",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.RemoteStoragePool

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}
