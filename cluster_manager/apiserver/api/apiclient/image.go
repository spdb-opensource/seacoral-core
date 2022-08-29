package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ ImageAPI = &clientConfig{}

type ImageAPI interface {
	PostImage(ctx context.Context, config api.ImageConfig) (api.TaskObjectResponse, error)
	DeleteImage(ctx context.Context, id string) error
	UpdateImage(ctx context.Context, id string, opts api.ImageOptions) (api.ObjectResponse, error)
	ListImages(ctx context.Context, id, imageType string) ([]api.Image, error)

	//template
	ListImageTemplates(ctx context.Context, name string) ([]api.ImageTemplatesResponse, error)
	UpdateImageTemplate(ctx context.Context, id string, conf api.ImageConfig) error
}

// NewImageAPI returns a ImageAPI
func NewImageAPI(host string, cli client.Client) ImageAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) PostImage(ctx context.Context, config api.ImageConfig) (api.TaskObjectResponse, error) {
	const uri = "/v1.0/manager/images"

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

func (c *clientConfig) DeleteImage(ctx context.Context, id string) error {
	uri := "/v1.0/manager/images/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

func (c *clientConfig) UpdateImage(ctx context.Context, id string, opts api.ImageOptions) (api.ObjectResponse, error) {
	uri := "/v1.0/manager/images/" + id

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

func (c *clientConfig) ListImages(ctx context.Context, id, imageType string) ([]api.Image, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if imageType != "" {
		params.Set("type", imageType)
	}

	url := url.URL{
		Path:     "/v1.0/manager/images",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Image

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}

func (c *clientConfig) ListImageTemplates(ctx context.Context, name string) ([]api.ImageTemplatesResponse, error) {
	params := make(url.Values)

	if name != "" {
		params.Set("name", name)
	}

	url := url.URL{
		Path:     "/v1.0/manager/images/templates",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.ImageTemplatesResponse

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}

func (c *clientConfig) UpdateImageTemplate(ctx context.Context, id string, conf api.ImageConfig) error {
	uri := "/v1.0/manager/images/templates/" + id

	resp, err := requireOK(c.client.Put(ctx, uri, conf))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return err
}
