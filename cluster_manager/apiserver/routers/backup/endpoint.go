package backup

import (
	"context"
	"encoding/json"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"net/http"
)

// swagger:parameters postBackupEndpoint
type postBackupEndpoint struct {
	// in: body
	// required: true
	Body api.BackupEndpoint
}

func (br backupRoute) createBackupEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/backup/endpoint backup endpoint
	//
	// 增加新备份终端
	//
	// Add a new　backup endpoint
	// This will create a new　backup endpoint
	//
	//     Responses:
	//       201: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.BackupEndpoint{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	obj, err := br.bankend.AddEndpoint(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, obj, nil
}

// swagger:parameters deleteBackupEndpoint
type deleteBackupEndpoint struct {
	// in: body
	// required: true
	//Body api.BackupEndpoint
}

func (br backupRoute) deleteBackupEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/backup/endpoint/{id} backup endpoint
	//
	// 删除一个备份终端
	//
	// Add a new　backup endpoint
	// This will delete a backup endpoint
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := vars["id"]

	err := br.bankend.DeleteEndpoint(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}

// swagger:parameters putBackupEndpoint
type putBackupEndpoint struct {
	// in: body
	// required: true
	Body api.BackupEndpoint
}

func (br backupRoute) updateBackupEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/backup/endpoint/{id} backup endpoint
	//
	// 增加新备份终端
	//
	// Add a new　backup endpoint
	// This will create a new　backup endpoint
	//
	//     Responses:
	//       201: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]
	req := api.BackupEndpoint{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	obj, err := br.bankend.UpdateEndpoint(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, obj, nil
}

// get a backup strategy info
//
// swagger:response getBackupEndpoint
type getBackupEndpoint struct {
	// in: body
	Body api.BackupEndpoint
}

func (br backupRoute) getBackupEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/backup/endpoint/{id} backup listBackupStrategy
	//
	// 查询一个备份终端
	//
	// Get a backup endpoints
	// This will returns a backup endpoint
	//
	//     Responses:
	//       200: getBackupEndpoint
	//       500: ErrorResponse

	id := vars["id"]

	backEndpoint, err := br.bankend.GetEndpoint(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, backEndpoint, nil
}

// list backup strategy info
//
// swagger:response listBackupEndpoint
type listBackupEndpoint struct {
	// in: body
	Body []api.BackupEndpoint
}

func (br backupRoute) listBackupEndpoints(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/backup/endpoint backup listBackupStrategy
	//
	// 查询备份终端
	//
	// List backup endpoints
	// This will returns a list of backup endpoint
	//
	//     Responses:
	//       200: listBackupEndpoint
	//       500: ErrorResponse

	siteId := r.FormValue("site_id")
	endpointType := r.FormValue("type")

	list, err := br.bankend.ListEndpoint(ctx, siteId, endpointType)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, []api.BackupEndpoint{}, nil
	}

	return http.StatusOK, list, nil
}
