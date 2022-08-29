package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ TaskAPI = &clientConfig{}

type TaskAPI interface {
	ListTasks(ctx context.Context, id, relatedId, action, status string) ([]api.Task, error)
}

func NewTaskAPI(host string, cli client.Client) TaskAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

func (c *clientConfig) ListTasks(ctx context.Context, id, relatedId, action, status string) ([]api.Task, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if relatedId != "" {
		params.Set("related_id", relatedId)
	}

	if action != "" {
		params.Set("action", action)
	}

	if status != "" {
		params.Set("status", status)
	}

	url := url.URL{
		Path:     "/v1.0/manager/tasks",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Task

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}
