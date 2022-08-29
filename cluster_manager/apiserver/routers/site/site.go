package site

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

const DefaultSiteConfigPath = "/root/.kube"

// lijj32(change log): function ConfigPath() hardcoded "/root" as the parent path of k8s config file,
// which is not generic for mac os and windows,
// therefore change it to a variable to allow users to specify the parent path with command line argument
// when starting CM-apiserver.
var siteConfigPath = DefaultSiteConfigPath

func init() {
	flag.StringVar(&siteConfigPath, "siteConfigPath", DefaultSiteConfigPath, "site config path")
}

func RegisterSiteRoute(bankend siteBankend, routers router.Adder) {
	r := &siteRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewPostRoute("/manager/sites", r.postSite),
		router.NewGetRoute("/manager/sites", r.listSites),
		router.NewPutRoute("/manager/sites/{id}", r.updateSite),
		router.NewDeleteRoute("/manager/sites/{id}", r.deleteSite),
	}

	routers.AddRouter(r)
}

type siteBankend interface {
	Add(ctx context.Context, config api.SiteConfig) (api.Site, error)
	Set(ctx context.Context, id string, opts api.SiteOptions) error
	List(ctx context.Context, id, name string) ([]api.Site, error)
	Delete(ctx context.Context, id string) error
}

type siteRoute struct {
	bankend siteBankend
	routes  []router.Route
}

func (sr siteRoute) Routes() []router.Route {
	return sr.routes
}

// swagger:parameters postSite
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.SiteConfig
}

func (sr siteRoute) postSite(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/sites sites postSite
	//
	// 增加新站点
	//
	// Add a new Site
	// This will create a new site
	//
	//     Responses:
	//       201: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.SiteConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if req.Path == "" {
		req.Path = ConfigPath(req.Domain, req.Port)
	}

	site, err := sr.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, api.ObjectResponse{
		ID:   site.ID,
		Name: site.Name,
	}, nil
}

func ConfigPath(domain string, port int) string {
	// lijj32: windows does not allow file name contains character ":", so change it to under-bar("_")
	return filepath.Join(siteConfigPath, fmt.Sprintf("%s_%d", domain, port))
}

// list object options
//
// swagger:parameters listSites
type listSiteRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`
}

// list sites info
//
// swagger:response listSitesResponseWrapper
type listSitesResponseWrapper struct {
	// in: body
	Body api.SitesResponse
}

func (sr siteRoute) listSites(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/sites sites listSites
	//
	// 查询站点
	//
	// List Site
	// This will returns a list of sites by options
	//
	//     Responses:
	//       200: listSitesResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	name := r.FormValue("name")

	list, err := sr.bankend.List(ctx, id, name)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.SitesResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// update object
//
// swagger:parameters updateSite
type updateSiteRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body api.SiteOptions
}

func (sr siteRoute) updateSite(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/sites/{id} sites updateSite
	//
	// 更改站点
	//
	// Update Site
	// This will update the site
	//
	//     Responses:
	//       200: description: OK
	//       500: ErrorResponse

	id := vars["id"]

	req := api.SiteOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if req.Domain != nil && req.Port != nil {
		req.Path = ConfigPath(*req.Domain, *req.Port)
	}

	err = sr.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

// object by id
//
// swagger:parameters deleteSite
type deleteSiteRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (sr siteRoute) deleteSite(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/sites/{id} sites deleteSite
	//
	// 删除站点
	//
	// Remote the Site
	// This will remove the site by id
	//
	//     Responses:
	//       200: description: OK
	//       500: ErrorResponse

	id := vars["id"]

	err := sr.bankend.Delete(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, nil, nil
}
