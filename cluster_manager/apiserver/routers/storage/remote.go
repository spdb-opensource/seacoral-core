package storage

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterStorageRoute(bankend storageBankend, routers router.Adder) {
	r := &sanStorageRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewPostRoute("/manager/storages/remote", r.postRemoteStorage),
		router.NewGetRoute("/manager/storages/remote", r.listRemoteStorages),
		router.NewPutRoute("/manager/storages/remote/{id}", r.updateRemoteStorage),
		router.NewDeleteRoute("/manager/storages/remote/{id}", r.deleteRemoteStorage),

		router.NewGetRoute("/manager/storages/remote/{storage}/pools", r.listRemoteStoragePools),
		router.NewPostRoute("/manager/storages/remote/{storage}/pools", r.postRemoteStoragePools),
		router.NewPutRoute("/manager/storages/remote/{storage}/pools/{pool}", r.updateRemoteStoragePool),
		router.NewDeleteRoute("/manager/storages/remote/{storage}/pools/{pool}", r.deleteRemoteStoragePool),
	}

	routers.AddRouter(r)
}

type storageBankend interface {
	storagePoolBankend

	Add(ctx context.Context, config api.RemoteStorageConfig) (api.RemoteStorage, error)
	List(ctx context.Context, id, name, site, enabled string) ([]api.RemoteStorage, error)
	Set(ctx context.Context, id string, opts api.RemoteStorageOptions) (api.RemoteStorage, error)
	Delete(ctx context.Context, id string) (api.TaskObjectResponse, error)
}

type sanStorageRoute struct {
	bankend storageBankend

	routes []router.Route
}

func (r sanStorageRoute) Routes() []router.Route {
	return r.routes
}

// swagger:parameters postRemoteStorage
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.RemoteStorageConfig
}

func (ss sanStorageRoute) postRemoteStorage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/storages/remote storages postRemoteStorage
	//
	// 增加新外部存储
	//
	// Add a new RemoteStorage
	// This will create a new remote storage
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.RemoteStorageConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	rs, err := ss.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.TaskObjectResponse{
			TaskID:     rs.Task.ID,
			ObjectID:   rs.ID,
			ObjectName: rs.Name,
		}, nil
}

// list object options
//
// swagger:parameters listRemoteStorages
type listRemoteStoragesRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`

	// in: query
	// required: false
	Site string `json:"site_id"`

	// in: query
	// required: false
	Enable string `json:"enabled"`
}

// list RemoteStorages info
//
// swagger:response listRemoteStoragesResponseWrapper
type listRemoteStoragesResponseWrapper struct {
	// in: body
	Body api.RemoteStoragesResponse
}

func (ss sanStorageRoute) listRemoteStorages(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/storages/remote storages listRemoteStorages
	//
	// 查询外部存储
	//
	// List RemoteStorages
	// This will returns a list of remote storage by options
	//
	//     Responses:
	//       200: listRemoteStoragesResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	name := r.FormValue("name")
	site := r.FormValue("site_id")
	enabled := r.FormValue("enabled")

	list, err := ss.bankend.List(ctx, id, name, site, enabled)
	if err != nil && len(list) == 0 {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.RemoteStoragesResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// update object
//
// swagger:parameters updateRemoteStorage
type updateRemoteStorageRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.RemoteStorageOptions
}

func (ss sanStorageRoute) updateRemoteStorage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/storages/remote/{id} storages updateRemoteStorage
	//
	// 更改外部存储
	//
	// Update RemoteStorage
	// This will update the remote storage
	//
	//     Responses:
	//       200: description: OK
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	req := api.RemoteStorageOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	rs, err := ss.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.TaskObjectResponse{
		ObjectID:   rs.ID,
		ObjectName: rs.Name,
		TaskID:     rs.Task.ID,
	}, nil
}

// object by id
//
// swagger:parameters deleteRemoteStorage
type deleteRemoteStorageRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (ss sanStorageRoute) deleteRemoteStorage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/storages/remote/{id} storages deleteRemoteStorage
	//
	// 删除外部存储
	//
	// Delete RemoteStorage
	// This will delete the remote storage
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := vars["id"]

	out, err := ss.bankend.Delete(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, out, nil
}
