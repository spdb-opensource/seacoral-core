package apiclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var _ AppAPI = &clientConfig{}

type AppAPI interface {
	//增加新服务
	AddApp(ctx context.Context, config api.AppConfig) (api.TaskObjectResponse, error)
	//增加数据库服务	TODO
	AddAppDBSchema(ctx context.Context, id string, config api.AppSchemaConfig) (api.TaskObjectResponse, error)
	//增加服务用户	TODO
	AddAppDBUser(ctx context.Context, id string, config api.AppUserConfig) (api.TaskObjectResponse, error)

	//查询服务	/manager/apps
	ListApps(ctx context.Context, id, name string) ([]api.Application, error)
	//查询服务	/manager/apps/{app}/database/schema	TODO
	ListAppDBSchema(ctx context.Context, app, id, name string) (api.DBSchemaResponse, error)
	//查询服务 	​/manager​/apps​/{app}​/database​/schema​/{schema}	TODO
	ListAppDBSchemaDetail(ctx context.Context, app, schema, id, name string) (api.DBSchemaDetailResponse, error)
	//查询数据库用户	/manager/apps/{app}/database/user	TODO
	ListAppDBUsers(ctx context.Context, app, id, name string) (api.AppUsersResponse, error)

	//删除服务	/manager/apps/{id}
	DeleteApp(ctx context.Context, id string) error
	//删除服务	/manager/apps/{app}/database/schema/{schema}	TODO
	DeleteAppDBSchema(ctx context.Context, id, schema string) error
	//删除服务数据库用户	TODO
	DeleteAppDBUser(ctx context.Context, id, user, ip string) error

	//更改服务架构
	UpdateAppArch(ctx context.Context, id string, archOpts api.AppArchOptions) (api.TaskObjectResponse, error)
	//更改服务镜像=版本变更
	UpdateAppImage(ctx context.Context, id string, imageOpts api.AppImageOptions) (api.TaskObjectResponse, error)
	//更改服务资源需求=资源请求变更
	UpdateAppResources(ctx context.Context, id string, resourcesOpts api.AppResourcesOptions) (api.TaskObjectResponse, error)
	//更改服务状态,启动或停止=运行状态变更
	UpdateAppState(ctx context.Context, id string, stateOpts api.AppStateOptions) (api.TaskObjectResponse, error)
	//更改服务单元状态,启动或停止 TODO
	UpdateUnitState(ctx context.Context, id, unit string, opts api.AppStateOptions) (api.TaskObjectResponse, error)
}

func NewAppAPI(host string, cli client.Client) AppAPI {
	return &clientConfig{
		host:   host,
		client: cli,
	}
}

//增加新服务
func (c *clientConfig) AddApp(ctx context.Context, config api.AppConfig) (api.TaskObjectResponse, error) {
	const uri = "/v1.0/manager/apps"

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

//增加数据库服务	TODO
func (c *clientConfig) AddAppDBSchema(ctx context.Context, id string, config api.AppSchemaConfig) (api.TaskObjectResponse, error) {

	return api.TaskObjectResponse{}, nil
}

//增加服务用户	TODO
func (c *clientConfig) AddAppDBUser(ctx context.Context, id string, config api.AppUserConfig) (api.TaskObjectResponse, error) {

	return api.TaskObjectResponse{}, nil
}

//删除服务	/manager/apps/{id}
func (c *clientConfig) DeleteApp(ctx context.Context, id string) error {
	uri := "/v1.0/manager/apps/" + id

	resp, err := requireOK(c.client.Delete(ctx, uri))
	if err != nil {
		return err
	}

	client.EnsureBodyClose(resp)

	return nil
}

//删除服务	/manager/apps/{app}/database/schema/{schema}	TODO
func (c *clientConfig) DeleteAppDBSchema(ctx context.Context, id, schema string) error {

	return nil
}

//删除服务数据库用户	TODO
func (c *clientConfig) DeleteAppDBUser(ctx context.Context, id, user, ip string) error {

	return nil
}

//查询服务
func (c *clientConfig) ListApps(ctx context.Context, id, name string) ([]api.Application, error) {
	params := make(url.Values)

	if id != "" {
		params.Set("id", id)
	}

	if name != "" {
		params.Set("name", name)
	}

	url := url.URL{
		Path:     "/v1.0/manager/apps",
		RawQuery: params.Encode(),
	}

	resp, err := requireOK(c.client.Get(ctx, url.RequestURI()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []api.Application

	err = decodeBody(resp, &list)
	if err != nil {
		return nil, errors.Errorf("%s %s%s,%v", http.MethodGet, c.host, resp.Request.URL.String(), err)
	}

	return list, err
}

//查询服务	/manager/apps/{app}/database/schema	TODO
func (c *clientConfig) ListAppDBSchema(ctx context.Context, app, id, name string) (api.DBSchemaResponse, error) {
	return nil, nil
}

//查询服务 	​/manager​/apps​/{app}​/database​/schema​/{schema}	TODO
func (c *clientConfig) ListAppDBSchemaDetail(ctx context.Context, app, schema, id, name string) (api.DBSchemaDetailResponse, error) {
	return api.DBSchemaDetailResponse{}, nil
}

//查询数据库用户	/manager/apps/{app}/database/user	TODO
func (c *clientConfig) ListAppDBUsers(ctx context.Context, app, id, name string) (api.AppUsersResponse, error) {
	return nil, nil
}

//更改服务架构
func (c *clientConfig) UpdateAppArch(ctx context.Context, id string, archOpts api.AppArchOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/apps/" + id + "/arch"

	resp, err := requireOK(c.client.Put(ctx, uri, archOpts))
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

//更改服务镜像=版本变更
func (c *clientConfig) UpdateAppImage(ctx context.Context, id string, imageOpts api.AppImageOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/apps/" + id + "/image"

	resp, err := requireOK(c.client.Put(ctx, uri, imageOpts))
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

//更改服务资源需求=资源请求变更
func (c *clientConfig) UpdateAppResources(ctx context.Context, id string, resourcesOpts api.AppResourcesOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/apps/" + id + "/resource/requests"

	resp, err := requireOK(c.client.Put(ctx, uri, resourcesOpts))
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

//更改服务状态,启动或停止=运行状态变更
func (c *clientConfig) UpdateAppState(ctx context.Context, id string, stateOpts api.AppStateOptions) (api.TaskObjectResponse, error) {
	uri := "/v1.0/manager/apps/" + id + "/state"

	resp, err := requireOK(c.client.Put(ctx, uri, stateOpts))
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

//更改服务单元状态,启动或停止 TODO
func (c *clientConfig) UpdateUnitState(ctx context.Context, id, unit string, opts api.AppStateOptions) (api.TaskObjectResponse, error) {

	return api.TaskObjectResponse{}, nil
}
