package app

import (
	"context"
	"encoding/json"
	"net/http"

	log "k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// swagger:parameters postApp
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.AppConfig
}

func (ar appRoute) postApp(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/apps apps postApp
	//
	// 增加新服务
	//
	// Add a new app
	// This will create a new appss
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse
	subscriptionId := r.FormValue("subscription_id")

	req := api.AppConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	app, err := ar.bankend.AddApp(ctx, req, subscriptionId)
	if err != nil {
		// lijj32: log error when AddApp() failed
		log.Error("add app failed.", err.Error())
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.TaskObjectResponse{
			TaskID:     app.Task.ID,
			ObjectID:   app.ID,
			ObjectName: app.Name,
		}, nil
}

// swagger:parameters postAppUser
type postUserRequest struct {
	// in: path
	// required: true
	App string `json:"app"`

	// in: body
	// required: true
	Body api.AppUserConfig
}

func (ar appRoute) postAppUser(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/apps/{app}/database/users apps postAppUser
	//
	// 增加服务用户
	//
	// Add a new app user
	// This will create a new apps user
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	app := vars["app"]
	req := api.AppUserConfig{}

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	task, err := ar.bankend.AddAppDBUser(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, task, nil
}

// swagger:parameters postAppDBSchema
type postDBSchemaRequest struct {
	// in: path
	// required: true
	App string `json:"app"`
	// in: body
	// required: true
	Body api.AppSchemaConfig
}

func (ar appRoute) postAppDBSchema(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/apps/{app}/database/schema apps postAppDBSchema
	//
	// 增加数据库服务
	//
	// Add a new app database
	// This will create a new apps database
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	subscriptionId := r.FormValue("subscription_id")
	app := vars["app"]
	req := api.AppSchemaConfig{}

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	task, err := ar.bankend.AddAppDBSchema(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, task, nil
}
