package host

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterClusterRoute(bankend clusterBankend, routers router.Adder) {
	r := &clusterRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewGetRoute("/manager/clusters", r.listClusters),
		router.NewPostRoute("/manager/clusters", r.postCluster),
		router.NewPutRoute("/manager/clusters/{id}", r.updateCluster),
		router.NewDeleteRoute("/manager/clusters/{id}", r.deleteCluster),
	}

	routers.AddRouter(r)
}

type clusterBankend interface {
	Add(ctx context.Context, config api.ClusterConfig) (api.Cluster, error)
	List(ctx context.Context, id, name, site, enabled string) ([]api.Cluster, error)
	Set(ctx context.Context, id string, opts api.ClusterOptions) (api.Cluster, error)
	Delete(ctx context.Context, id string) error
}

type clusterRoute struct {
	bankend clusterBankend

	routes []router.Route
}

func (r clusterRoute) Routes() []router.Route {
	return r.routes
}

// swagger:parameters postCluster
type postClusterOptionsRequest struct {
	// in: body
	// required: true
	Body api.ClusterConfig
}

func (cr clusterRoute) postCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/clusters clusters postCluster
	//
	// 增加新集群
	//
	// Add a new Cluster
	// This will create a new cluster
	//
	//     Responses:
	//       201: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.ClusterConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	cluster, err := cr.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated,
		api.ObjectResponse{
			ID:   cluster.ID,
			Name: cluster.Name,
		}, nil
}

// list object options
//
// swagger:parameters listClusters
type listClusterRequest struct {
	// in: query
	// required: false
	ID bool `json:"id"`

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

// list clusters info
//
// swagger:response listClusterResponseWrapper
type listClusterResponseWrapper struct {
	// in: body
	Body api.ClustersResponse
}

func (cr clusterRoute) listClusters(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/clusters clusters listClusters
	//
	// 查询集群
	//
	// List Clusters
	// This will return a list of clusters by conditions
	//
	//     Responses:
	//       200: listClusterResponseWrapper
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := r.FormValue("id")
	name := r.FormValue("name")
	site := r.FormValue("site_id")
	enabled := r.FormValue("enabled")

	clusters, err := cr.bankend.List(ctx, id, name, site, enabled)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if clusters == nil {
		return http.StatusOK, api.ClustersResponse{}, nil
	}

	return http.StatusOK, clusters, nil
}

// update object
//
// swagger:parameters updateCluster
type updateClusterRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.ClusterConfig
}

func (cr clusterRoute) updateCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/clusters/{id} clusters updateCluster
	//
	// 更新集群
	//
	// Update a Cluster
	// This will update a cluster
	//
	//     Responses:
	//       200: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	req := api.ClusterOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	c, err := cr.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.ObjectResponse{
		ID:   c.ID,
		Name: c.Name,
	}, nil
}

// object by id
//
// swagger:parameters deleteCluster
type clusterNameInPathRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (cr clusterRoute) deleteCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/clusters/{id} clusters deleteCluster
	//
	// 出库集群
	//
	// Remove a Cluster
	// This will remove a cluster
	//
	//     Responses:
	//       204: description: Deleted
	//       500: ErrorResponse

	id := vars["id"]

	err := cr.bankend.Delete(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
