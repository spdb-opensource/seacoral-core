package app

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// update object
//
// swagger:parameters updateAppImage
type updateAppImageRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.AppImageOptions
}

func (ar appRoute) updateAppImage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/image apps updateAppImage
	//
	// 更改服务镜像
	//
	// Update the app image
	// This will update the app image
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]

	req := api.AppImageOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateImage(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters updateAppResources
type updateAppResourceRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.AppResourcesOptions
}

func (ar appRoute) updateAppResources(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/resource/requests apps updateAppResources
	//
	// 更改服务资源需求
	//
	// Update the app resources requests
	// This will update the app resources requests
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]

	req := api.AppResourcesOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateAppResourceRequests(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters updateAppArch
type updateAppArchRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.AppArchOptions
}

func (ar appRoute) updateAppArch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/arch apps updateAppArch
	//
	// 更改服务l架构
	//
	// Update the app arch requests
	// This will update the app arch requests
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppArchOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateArch(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters updateAppState
type updateAppStateRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.AppStateOptions
}

func (ar appRoute) updateAppState(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/state apps updateAppState
	//
	// 更改服务状态，启动或停止
	//
	// Update the app state
	// This will update the app state
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppStateOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateState(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters updateUnitState
type updateUnitStateRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: path
	// required: true
	Unit string `json:"unit"`

	// in: body
	// required: true
	Body api.AppStateOptions
}

func (ar appRoute) updateUnitState(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/units/{unit}/state apps updateUnitState
	//
	// 更改服务单元状态，启动或停止
	//
	// Update the app  unit state
	// This will update the app unit state
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	unit := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppStateOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateUnitState(ctx, app, unit, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters migrateUnit
type updateUnitMigrateRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: path
	// required: true
	Unit string `json:"unit"`

	// in: body
	// required: true
	Body api.UnitMigrateOptions
}

func (ar appRoute) migrateUnit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/units/{unit}/migrate apps migrateUnit
	//
	// 迁移单元
	//
	// migrate the app  unit
	// This will migrate the app unit
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	unit := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.UnitMigrateOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UnitMigrate(ctx, app, unit, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters rebuildUnit
type updateUnitRebuildRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: path
	// required: true
	Unit string `json:"unit"`

	// in: body
	// required: true
	Body api.UnitRebuildOptions
}

func (ar appRoute) rebuildUnit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/units/{unit}/rebuild apps rebuildUnit
	//
	// 重建服务单元
	//
	// rebuild the app  unit
	// This will rebuild the app unit
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	unit := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.UnitRebuildOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UnitRebuild(ctx, app, unit, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters restoreUnit
type updateUnitRestoreRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: path
	// required: true
	Unit string `json:"unit"`

	// in: body
	// required: true
	Body api.UnitRestoreOptions
}

func (ar appRoute) restoreUnit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/units/{unit}/restore apps restoreUnit
	//
	// 备份数据恢复单元
	//
	// restore the app  unit
	// This will restore the app unit
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	unit := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.UnitRestoreOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UnitRestore(ctx, app, unit, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters restoreUnit
type updateUnitResourcesRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: path
	// required: true
	Unit string `json:"unit"`

	// in: body
	// required: true
	Body api.AppResourcesOptions
}

func (ar appRoute) updateUnitResources(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/resource/requests apps updateAppResources
	//
	// 更改服务资源需求
	//
	// Update the app resources requests
	// This will update the app resources requests
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	unit := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppResourcesOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp, err := ar.bankend.UpdateUnitResourceRequests(ctx, app, unit, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, resp, nil
}

// update object
//
// swagger:parameters resetAppUserPassword
type updateAppUserPasswordRequest struct {
	// in: path
	// required: true
	App string `json:"app"`

	// in: body
	// required: true
	Body api.AppUserResetConfig
}

func (ar appRoute) resetAppUserPassword(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{app}/database/users/pwd apps resetAppUserPassword
	//
	// 重置服务用户密码
	//
	// Reset the app user password
	// This will reset the app user password
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppUserResetConfig{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.ResetAppDBUser(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) roleSwitch(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{app}/role  apps roleSwitch
	//
	// 主从切换
	//
	// Switch the unit's role
	// This will switch the unit's role
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.UnitRoleSwitchConfig{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.RoleSwitch(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) updateConfig(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{app}/config  apps updateConfig
	//
	// 配置文件编辑
	//
	// Update config map
	// This will update the app config
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.ConfigMapOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.UpdateConfig(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) updateAppUserPrivileges(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{app}/database/users/privileges  apps updateAppUserPrivileges
	//
	// 数据库服务用户权限编辑
	//
	// Update app user privileges
	// This will update the user privileges
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	app := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, app, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.AppUserPrivilegesOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.UpdateUserPrivileges(ctx, app, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) setUnitRole(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/apps/{id}/units/{unit}/role  unit setUnitRole
	//
	// 服务单元角色设置
	//
	// Set unit role
	// This will set unit role
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	appID := vars["app"]
	unitID := vars["unit"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, appID, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.RoleSetOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.UnitRoleSet(ctx, appID, unitID, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) setCmhaReplMode(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/services/{id}/replication_mode  cmha set replication_mode
	//
	// cmha复制模式设置
	//
	// Set replication_mode
	// This will set replication_mode
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	appID := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, appID, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.CmhaReplModeOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.SetCmhaReplMode(ctx, appID, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) setCmhaMaintenance(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/services/{id}/maintenance  cmha set maintenance
	//
	// cmha维护状态变更
	//
	// Set Maintenance
	// This will set maintenance
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	appID := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, appID, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.CmhaMaintenanceOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.SetCmhaMaintenance(ctx, appID, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

func (ar appRoute) setReplSource(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/services/{id}/replication/set_source cmha set source
	//
	// cmha:MySQL复制源设置
	//
	// Set replication source
	// This will set replication source
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	appID := vars["app"]
	subscriptionId := r.FormValue("subscription_id")

	err := ar.bankend.CheckAppAndSubscription(ctx, appID, subscriptionId)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	req := api.CmhaReplModeOptions{}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = ar.bankend.CmhaReplSetSource(ctx, appID, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}
