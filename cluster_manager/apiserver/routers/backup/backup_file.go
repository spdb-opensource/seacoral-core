package backup

import (
	"context"
	"errors"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

// list object options
//
// swagger:parameters listBackupFiles
type listBackupFilesRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Unit string `json:"unit_id"`

	// in: query
	// required: false
	App string `json:"app_id"`

	// in: query
	// required: false
	Site string `json:"site_id"`
}

// list Apps info
//
// swagger:response listBackupFilesResponseWrapper
type listBackupFilesResponseWrapper struct {
	// in: body
	Body api.BackupFilesResponse
}

func (br backupRoute) listBackupFiles(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/backup/files backup listBackupFiles
	//
	// 查询备份文件
	//
	// List backup files
	// This will returns a list of backup files
	//
	//     Responses:
	//       200: listBackupFilesResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	unit := r.FormValue("unit_id")
	app := r.FormValue("app_id")
	site := r.FormValue("site_id")
	createdUser := r.FormValue("created_user")

	list, err := br.bankend.ListBackupFiles(ctx, id, unit, app, site, createdUser)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.BackupFilesResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// object by id
//
// swagger:parameters deleteBackupFile
type deleteBackupFileRequest struct {
	// 对象 ID 或者 Name
	//
	// required: false
	// in: query
	ID string `json:"id"`

	// required: false
	// in: query
	App string `json:"app_id"`
}

func (br backupRoute) deleteBackupFile(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/backup/files backup deleteBackupFile
	//
	// 删除备份文件
	//
	// Delete the backup　file
	// This will delete the backup file
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := r.FormValue("id")
	app := r.FormValue("app_id")

	if id == "" && app == "" {
		return http.StatusBadRequest, nil, errors.New("id or app_id is required in delete backup files")
	}

	err := br.bankend.DeleteBackupFile(ctx, id, app)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
