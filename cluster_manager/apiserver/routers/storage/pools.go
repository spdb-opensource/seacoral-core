package storage

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

type storagePoolBankend interface {
	AddPool(ctx context.Context, storage string, config api.RemoteStoragePoolConfig) (api.RemoteStoragePool, error)
	ListPools(ctx context.Context, storage string, id, name string) ([]api.RemoteStoragePool, error)
	SetPool(ctx context.Context, storage string, id string, opts api.RemoteStoragePoolOptions) (api.RemoteStoragePool, error)
	DeletePool(ctx context.Context, storage string, id string) (api.TaskObjectResponse, error)
}

// list object options
//
// swagger:parameters listRemoteStoragePools
type listRemoteStoragePoolsRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`

	// in: path
	// required: true
	Storage string `json:"storage"`
}

// list RemoteStorage pools info
//
// swagger:response listRemoteStoragePoolsResponseWrapper
type listRemoteStoragePoolsResponseWrapper struct {
	// in: body
	Body api.RemoteStoragePoolsResponse
}

func (ss sanStorageRoute) listRemoteStoragePools(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/storages/remote/{storage}/pools storages listRemoteStoragePools
	//
	// 查询外部存储池
	//
	// List RemoteStorage pools
	// This will returns a list of remote storage pools by options
	//
	//     Responses:
	//       200: listRemoteStoragePoolsResponseWrapper
	//       500: ErrorResponse

	storage := vars["storage"]
	id := r.FormValue("id")
	name := r.FormValue("name")

	list, err := ss.bankend.ListPools(ctx, storage, id, name)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.RemoteStoragePoolsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// swagger:parameters postRemoteStoragePools
type postPoolOptionsRequest struct {
	// in: path
	// required: true
	Storage string `json:"storage"`

	// in: body
	// required: true
	Body api.RemoteStoragePoolConfig
}

func (ss sanStorageRoute) postRemoteStoragePools(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/storages/remote/{storage}/pools storages postRemoteStoragePools
	//
	// 增加新外部存储池
	//
	// Add a new Remote Storage Pool
	// This will create a new remote storage poll
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	storage := vars["storage"]

	req := api.RemoteStoragePoolConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	pool, err := ss.bankend.AddPool(ctx, storage, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.TaskObjectResponse{
			TaskID:     pool.Task.ID,
			ObjectID:   pool.ID,
			ObjectName: pool.Name,
		}, nil
}

// update object
//
// swagger:parameters updateRemoteStoragePool
type updateRemoteStoragePoolRequest struct {
	// in: path
	// required: true
	Storage string `json:"storage"`

	// in: path
	// required: true
	Pool string `json:"pool"`

	// in: body
	// required: true
	Body api.RemoteStoragePoolOptions
}

func (ss sanStorageRoute) updateRemoteStoragePool(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/storages/remote/{storage}/pools/{pool} storages updateRemoteStoragePool
	//
	// 更改外部存储池
	//
	// Update RemoteStorage pool
	// This will update the remote storage pool
	//
	//     Responses:
	//       200: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	storage := vars["storage"]
	pool := vars["pool"]

	req := api.RemoteStoragePoolOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ss.bankend.SetPool(ctx, storage, pool, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.ObjectResponse{
		ID:   out.ID,
		Name: out.Name,
	}, nil
}

// object by pool id
//
// swagger:parameters deleteRemoteStoragePool
type deleteRemoteStoragePoolRequest struct {
	// in: path
	// required: true
	Storage string `json:"storage"`

	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	Pool string `json:"pool"`
}

func (ss sanStorageRoute) deleteRemoteStoragePool(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/storages/remote/{storage}/pools/{pool} storages deleteRemoteStoragePool
	//
	// 删除指定外部存储池
	//
	// Delete Remote Storage Pool
	// This will delete the remote storage pool
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	storage := vars["storage"]
	pool := vars["pool"]

	out, err := ss.bankend.DeletePool(ctx, storage, pool)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, out, nil
}
