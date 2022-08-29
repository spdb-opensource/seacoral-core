package app

import (
	"context"
	"net/http"
)

// object by id
//
// swagger:parameters deleteApp
type deleteAppRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (ar appRoute) deleteApp(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/apps/{id} apps deleteApp
	//
	// 删除服务
	//
	// Delete the app
	// This will delete the app
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ar.bankend.DeleteApp(ctx, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, out, nil
}

// object by id
//
// swagger:parameters deleteAppDBSchema
type deleteAppDBSchemaRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	App string `json:"app"`

	// required: true
	// in: path
	Schema string `json:"schema"`
}

func (ar appRoute) deleteAppDBSchema(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/apps/{app}/database/schema/{schema} apps deleteAppDBSchema
	//
	// 删除服务
	//
	// Delete the app schema
	// This will delete the app db schema
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	app := vars["app"]
	schema := vars["schema"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.DeleteAppDBSchema(ctx, app, schema)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}

// object by id
//
// swagger:parameters deleteAppDBUser
type deleteAppDBUserRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	App string `json:"app"`

	// required: true
	// in: path
	User string `json:"user"`

	// required: false
	// in: query
	IP string `json:"ip"`
}

func (ar appRoute) deleteAppDBUser(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/apps/{app}/database/users/{user} apps deleteAppDBUser
	//
	// 删除服务数据库用户
	//
	// Delete the app db user
	// This will delete the app db user
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	app := vars["app"]
	user := vars["user"]
	ip := r.FormValue("ip")
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.DeleteAppDBUser(ctx, app, user, ip)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
