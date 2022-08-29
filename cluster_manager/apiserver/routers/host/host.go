package host

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterHostRoute(bankend hostBankend, routers router.Adder) {
	r := &nodeRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewGetRoute("/manager/hosts", r.listHosts),
		router.NewPostRoute("/manager/hosts", r.postHost),
		router.NewPutRoute("/manager/hosts/{id}", r.updateHost),
		router.NewDeleteRoute("/manager/hosts/{id}", r.deleteHost),

		router.NewGetRoute("/manager/hosts/{id}/detail", r.getHostDetail),

		//验证username,password,ssh_port
		router.NewPostRoute("/manager/hosts/validation", r.validateHost),
	}

	routers.AddRouter(r)
}

type hostBankend interface {
	Add(ctx context.Context, config api.HostConfig) (api.Host, error)
	List(ctx context.Context, id, name, cluster, site, enabled string) ([]api.Host, error)
	Set(ctx context.Context, id string, opts api.HostOptions) (api.Host, error)

	GetDetail(ctx context.Context, id string) (api.HostDetail, error)
	Delete(ctx context.Context, id, user, password string, port int) (api.TaskObjectResponse, error)

	ValidateHost(ctx context.Context, config api.HostConfig, checkType string) error
}

type nodeRoute struct {
	bankend hostBankend

	routes []router.Route
}

func (r nodeRoute) Routes() []router.Route {
	return r.routes
}

// swagger:parameters postHost
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.HostConfig
}

func (nr nodeRoute) postHost(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/hosts hosts postHost
	//
	// 增加新计算节点
	//
	// Add a new Host
	// This will create a new host
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.HostConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	host, err := nr.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.TaskObjectResponse{
			TaskID:     host.Task.ID,
			ObjectID:   host.ID,
			ObjectName: host.Node.Name,
		}, nil
}

// list object options
//
// swagger:parameters listHosts
type listHostRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`

	// in: query
	// required: false
	Cluster string `json:"cluster_id"`

	// in: query
	// required: false
	Site string `json:"site_id"`

	// in: query
	// required: false
	Enable string `json:"enabled"`
}

// list hosts info
//
// swagger:response listHostResponseWrapper
type listHostResponseWrapper struct {
	// in: body
	Body api.HostsResponse
}

func (nr nodeRoute) getHostDetail(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/hosts/{id}/detail  hosts getHostsDetail
	//
	//查询计算节点详细信息
	//
	//Get Host Detail
	//This will return a host detail
	//
	//     Responses:
	//       200: listHostResponseWrapper
	//       400: ErrorResponse
	//       500: ErrorResponse
	id := vars["id"]

	hostDetail, err := nr.bankend.GetDetail(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, hostDetail, nil
}

func (nr nodeRoute) listHosts(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/hosts hosts listHosts
	//
	// 查询计算节点信息
	//
	// List Hosts
	// This will return a list of hosts by conditions
	//
	//     Responses:
	//       200: listHostResponseWrapper
	//       400: ErrorResponse
	//       500: ErrorResponse
	id := r.FormValue("id")
	name := r.FormValue("name")
	cluster := r.FormValue("cluster_id")
	site := r.FormValue("site_id")
	enabled := r.FormValue("enabled")

	list, err := nr.bankend.List(ctx, id, name, cluster, site, enabled)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.HostsResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// update object
//
// swagger:parameters updateHost
type updateHostRequest struct {
	// in: path
	// required: false
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.HostOptions
}

func (nr nodeRoute) updateHost(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/hosts/{id} hosts updateHost
	//
	// 更新计算节点信息
	//
	// Update a Host
	// This will update a host
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	req := api.HostOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	host, err := nr.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.TaskObjectResponse{
		ObjectID:   host.ID,
		ObjectName: host.Node.Name,
		TaskID:     host.Task.ID,
	}, nil
}

// object by id
//
// swagger:parameters deleteHost
type objectByNameInPathRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (nr nodeRoute) deleteHost(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/hosts/{id} hosts deleteHost
	//
	// 出库计算节点信息
	//
	// Remove a Host
	// This will remove a host
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := vars["id"]
	user := r.FormValue("username")
	password := r.FormValue("pwd")
	portStr := r.FormValue("ssh_port")

	port := 22
	if portStr != "" {
		portInt, err := strconv.Atoi(portStr)
		if err != nil {
			return http.StatusInternalServerError, nil, err
		}
		port = portInt
	}

	out, err := nr.bankend.Delete(ctx, id, user, password, port)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, out, nil
}

func (nr nodeRoute) validateHost(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/hosts/validation hosts legalizeHost
	//
	// 校验计算节点
	//
	// Legalize a new Host
	// This will validate a new host
	//
	//     Responses:
	//       201: StatusCreated:
	//       400: ErrorResponse
	//       500: ErrorResponse
	checkType := r.FormValue("type")
	if checkType == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("checkType is required")
	}

	req := api.HostConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	err = nr.bankend.ValidateHost(ctx, req, checkType)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, nil, nil
}
