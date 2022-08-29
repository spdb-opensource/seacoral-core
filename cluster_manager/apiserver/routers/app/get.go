package app

import (
	"context"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// list object options
//
// swagger:parameters listApps,listAppsDetail
type listAppsRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`
}

// list Apps info
//
// swagger:response listAppsResponseWrapper
type listAppsResponseWrapper struct {
	// in: body
	Body api.AppsResponse
}

func (ar appRoute) listApps(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps apps listApps
	//
	// 查询服务
	//
	// List apps
	// This will returns a list of apps
	//
	//     Responses:
	//       200: listAppsResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	id := r.FormValue("id")
	name := r.FormValue("name")

	list, err := ar.bankend.ListApps(ctx, id, name, subscriptionId, false)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.AppsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

func (ar appRoute) listAppsDetail(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/detail apps listAppsDetail
	//
	// 查询服务
	//
	// List apps
	// This will returns a list of apps
	//
	//     Responses:
	//       200: listAppsResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	id := r.FormValue("id")
	name := r.FormValue("name")

	list, err := ar.bankend.ListApps(ctx, id, name, subscriptionId, true)
	if len(list) > 0 {
		return http.StatusOK, list, nil
	}
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.AppsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// list object options
//
// swagger:parameters listAppDBSchema
type listAppDBSchemaRequest struct {
	// in: path
	// required: true
	App string `json:"app"`
}

// list App DB Schema
//
// swagger:response listAppDBSchemaResponseWrapper
type listAppDBSchemaResponseWrapper struct {
	// in: body
	Body api.DBSchemaResponse
}

func (ar appRoute) listAppDBSchema(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/{app}/database/schema apps listAppDBSchema
	//
	// 查询服务
	//
	// List app db schema
	// This will returns a list of app db schema
	//
	//     Responses:
	//       200: listAppDBSchemaResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	app := vars["app"]

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	list, err := ar.bankend.ListAppDBSchema(ctx, app)
	if len(list) > 0 {
		return http.StatusOK, list, nil
	}
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.AppsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// list object options
//
// swagger:parameters listAppDBSchemaDetail
type listAppDBSchemaDetailRequest struct {
	// in: path
	// required: true
	App string `json:"app"`

	// in: path
	// required: true
	Schema string `json:"schema"`

	// in: query
	// required: false
	ID string `json:"id"`
	// in: query
	// required: false
	Name string `json:"name"`
}

// list App DB Schema detail
//
// swagger:response listAppDBSchemaDetailResponseWrapper
type listAppDBSchemaDetailResponseWrapper struct {
	// in: body
	Body api.DBSchemaDetailResponse
}

func (ar appRoute) listAppDBSchemaDetail(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/{app}/database/schema/{schema} apps listAppDBSchemaDetail
	//
	// 查询服务
	//
	// List app db schema detail
	// This will returns a list of app db schema detail
	//
	//     Responses:
	//       200: listAppDBSchemaDetailResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	app := vars["app"]
	schema := vars["schema"]

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ar.bankend.ListAppDBSchemaDetail(ctx, app, schema)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

// list object options
//
// swagger:parameters listAppDBUser
type listAppDBUserRequest struct {
	// in: path
	// required: true
	App string `json:"app"`
}

// list App DB User
//
// swagger:response listAppDBUsersResponseWrapper
type listAppDBUsersResponseWrapper struct {
	// in: body
	Body api.AppUsersResponse
}

func (ar appRoute) listAppDBUser(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/{app}/database/users apps listAppDBUser
	//
	// 查询数据库用户
	//
	// List app db user
	// This will returns a list of app db user
	//
	//     Responses:
	//       200: listAppDBUsersResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	app := vars["app"]

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	list, err := ar.bankend.ListAppDBUsers(ctx, app)
	if len(list) > 0 {
		return http.StatusOK, list, nil
	}
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.AppsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// list object options
//
// swagger:parameters listAppDBUser
type listAppDBSingleUserRequest struct {
	// in: path
	// required: true
	App string `json:"app"`

	// in: path
	// required: true
	User string `json:"user"`

	// in: query
	// required: true
	IP string `json:"ip"`
}

// list App DB User
//
// swagger:response listAppDBUserResponseWrapper
type listAppDBUserResponseWrapper struct {
	// in: body
	Body api.DatabaseUser
}

func (ar appRoute) listAppDBSingleUser(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/{app}/database/users/{user} apps listAppDBSingleUser
	//
	// 查询数据库用户
	//
	// List app db user
	// This will returns a assigned app db user
	//
	//     Responses:
	//       200: listAppDBUserResponseWrapper
	//       500: ErrorResponse

	app := vars["app"]
	user := vars["user"]
	ip := r.FormValue("ip")
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ar.bankend.GetAppDBUser(ctx, app, user, ip)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

func (ar appRoute) listConfig(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps/{app}/config   apps listConfig
	//
	// 查询配置文件
	//
	// List config
	// This will returns configmap
	//
	//     Responses:
	//       200: listAppDBUserResponseWrapper
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ar.bankend.ListConfig(ctx, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

func (ar appRoute) listAppsWithPagination(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/apps apps listApps
	//
	// 查询服务
	//
	// List apps
	// This will returns a list of apps
	//
	//     Responses:
	//       200: listAppsResponseWrapper
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	id := r.FormValue("id")
	name := r.FormValue("name")
	paginationReq := api.NewPaginationReq(r.FormValue("size"), r.FormValue("page"))

	res, err := ar.bankend.ListAppsWithPagination(ctx, id, name, subscriptionId, false, paginationReq)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, res, nil
}

func (ar appRoute) getCmhaTopology(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/services/{id}/topology  show cmha service topology
	//
	// 查询cmha服务topology
	//
	// Show cmha service topology
	// This will returns cmha service topology
	//
	//     Responses:
	//       200: listAppsResponseWrapper
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ar.bankend.GetCmhaTopology(ctx, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

//func (ar appRoute) getJsonSchema(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
//	// swagger:route GET /manager/jsonschema/{schema} getJsonSchema
//	//
//	// 查询服务
//	//
//	// List apps
//	// This will returns a json schema
//	//
//	//     Responses:
//	//       200: listAppsResponseWrapper
//	//       500: StatusNotFound
//
//	schema := vars["schema"]
//
//	jsonSchema, err := api.GetJsonSchema(schema + ".json")
//	if err != nil {
//		return http.StatusNotFound, nil, err
//	}
//
//	return http.StatusOK, string(jsonSchema), nil
//}
