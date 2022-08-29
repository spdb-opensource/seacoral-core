package backup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// swagger:parameters postStrategy
type postStrategyRequest struct {
	// in: body
	// required: true
	Body api.BackupStrategyConfig
}

func (br backupRoute) postStrategy(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/backup/strategy backup postStrategy
	//
	// 增加新服务备份策略
	//
	// Add a new app　backup strategy
	// This will create a new app backup strategy
	//
	//     Responses:
	//       201: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.BackupStrategyConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	obj, err := br.bankend.AddBackupStrategy(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.ObjectResponse{
			ID:   obj.ID,
			Name: obj.Name,
		}, nil
}

// update object
//
// swagger:parameters updateStrategy
type updateBackupStrategyRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.BackupStrategyOptions
}

func (br backupRoute) updateStrategy(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/backup/strategy/{id} backup updateStragety
	//
	// 更改服务备份策略
	//
	// Update the app backup strategy
	// This will update the app backup strategy
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	req := api.BackupStrategyOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = br.bankend.SetBackupStrategy(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

// list object options
//
// swagger:parameters listBackupFiles
type listBackupStrategyRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Unit string `json:"unit_id"`

	// in: query
	// required: false
	App string `json:"app_id"`
}

// list backup strategy info
//
// swagger:response listBackupStrategyResponseWrapper
type listBackupStrategyResponseWrapper struct {
	// in: body
	Body api.BackupStrategyResponse
}

func (br backupRoute) listBackupStrategy(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/backup/strategy backup listBackupStrategy
	//
	// 查询备份策略
	//
	// List backup strategy
	// This will returns a list of backup strategy
	//
	//     Responses:
	//       200: listBackupStrategyResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	unit := r.FormValue("unit_id")
	app := r.FormValue("app_id")

	list, err := br.bankend.ListBackupStrategy(ctx, id, unit, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.BackupStrategyResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// object by id
//
// swagger:parameters deleteBackupStrategy
type deleteBackupStrategyRequest struct {
	// 对象 ID 或者 Name
	//
	// required: false
	// in: query
	ID string `json:"id"`

	// required: false
	// in: query
	App string `json:"app_id"`
}

func (br backupRoute) deleteBackupStrategy(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/backup/strategy backup deleteBackupStrategy
	//
	// 删除备份策略
	//
	// Delete the backup　strategy
	// This will delete the backup strategy
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := r.FormValue("id")
	app := r.FormValue("app_id")

	if id == "" && app == "" {
		return http.StatusBadRequest, nil, errors.New("id or app_id is required in delete backup strategy")
	}

	err := br.bankend.DeleteBackupStrategy(ctx, id, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
