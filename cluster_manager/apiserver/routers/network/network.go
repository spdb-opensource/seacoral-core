package network

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterNetworkRoute(bankend networkBankend, routers router.Adder) {
	r := &networkRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewPostRoute("/manager/networks", r.postNetwork),
		router.NewGetRoute("/manager/networks", r.listNetworks),
		router.NewPutRoute("/manager/networks/{id}", r.updateNetwork),
		router.NewDeleteRoute("/manager/networks/{id}", r.deleteNetwork),
	}

	routers.AddRouter(r)
}

type networkBankend interface {
	Add(ctx context.Context, config api.NetworkConfig) (api.Network, error)
	List(ctx context.Context, id, name, cluster, site, topology, enabled string) ([]api.Network, error)
	Set(ctx context.Context, id string, opts api.NetworkOptions) (api.Network, error)
	Delete(ctx context.Context, id string) error
}

type networkRoute struct {
	bankend networkBankend

	routes []router.Route
}

func (r networkRoute) Routes() []router.Route {
	return r.routes
}

// swagger:parameters postNetwork
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.NetworkConfig
}

func (nr networkRoute) postNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/networks networks postNetwork
	//
	// 增加新网段
	//
	// Add a new Network
	// This will create a new network
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse
	req := api.NetworkConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	network, err := nr.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.ObjectResponse{
			ID:   network.ID,
			Name: network.Name,
		}, nil
}

// list object options
//
// swagger:parameters listNetworks
type listNetworskRequest struct {

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
	Topology string `json:"topology"`

	// in: query
	// required: false
	Enable string `json:"enabled"`
}

// list object info
//
// swagger:response listNetworksResponseWrapper
type listNetworksResponseWrapper struct {
	// in: body
	Body api.NetworksResponse
}

func (nr networkRoute) listNetworks(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/networks networks listNetworks
	//
	// 查询新网段
	//
	// List Networks
	// This will returns a list of networks by options
	//
	//     Responses:
	//       200: listNetworksResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	name := r.FormValue("name")
	site := r.FormValue("site_id")
	cluster := r.FormValue("cluster_id")
	topology := r.FormValue("topology")
	enabled := r.FormValue("enabled")

	list, err := nr.bankend.List(ctx, id, name, cluster, site, topology, enabled)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.NetworksResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// update object
//
// swagger:parameters updateNetwork
type updateNetworkRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.NetworkOptions
}

func (nr networkRoute) updateNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/networks/{id} networks updateNetwork
	//
	// 更改网段
	//
	// Update Networks
	// This will update the network
	//
	//     Responses:
	//       200: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]
	req := api.NetworkOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	net, err := nr.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.ObjectResponse{
		ID:   net.ID,
		Name: net.Name,
	}, nil
}

// object by id
//
// swagger:parameters deleteNetwork
type deleteImageRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (nr networkRoute) deleteNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/networks/{id} networks deleteNetwork
	//
	// 更改网段
	//
	// Update Networks
	// This will update the network
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := vars["id"]

	err := nr.bankend.Delete(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
