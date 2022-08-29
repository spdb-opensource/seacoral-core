package image

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterImageRoute(bankend imageBankend, routers router.Adder) {
	r := &imageRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewGetRoute("/manager/images", r.listImages),
		router.NewPostRoute("/manager/images", r.postImage),
		router.NewPutRoute("/manager/images/{id}", r.updateImage),
		router.NewDeleteRoute("/manager/images/{id}", r.deleteImage),

		router.NewGetRoute("/manager/images/{id}/templates", r.listImageTemplates),
		router.NewPutRoute("/manager/images/{id}/templates", r.updateImageTemplate),

		router.NewGetRoute("/manager/images/{id}/scripts", r.listImageScripts),
		router.NewPutRoute("/manager/images/{id}/scripts", r.syncImageScripts),
		// DBCH-TOREMOVE remove maintenance
		router.NewPutRoute("/maintenance/images/{id}/scripts", r.syncImageScripts),
	}

	routers.AddRouter(r)
}

type imageBankend interface {
	Add(ctx context.Context, config api.ImageConfig) (api.Image, error)
	List(ctx context.Context, opts api.ImageListOptions) ([]api.Image, error)
	Set(ctx context.Context, id string, opts api.ImageOptions) (api.Image, error)
	Delete(ctx context.Context, id string) (api.TaskObjectResponse, error)

	ListImageTemplates(ctx context.Context, id string) (api.ImageTemplate, error)
	SetImageTemplate(ctx context.Context, id string, kvs []api.KeySet) (api.TaskObjectResponse, error)

	ListImageScripts(ctx context.Context, id string) (api.ImageScripts, error)
	SyncImageScripts(ctx context.Context, id string, scriptType string) error
}

type imageRoute struct {
	bankend imageBankend

	routes []router.Route
}

func (ir imageRoute) Routes() []router.Route {
	return ir.routes
}

// swagger:parameters postImage
type postOptionsRequest struct {
	// in: body
	// required: true
	Body api.ImageConfig
}

func (ir imageRoute) postImage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route POST /manager/images images postImage
	//
	// 增加新镜像
	//
	// Add a new Image
	// This will create a new image
	//
	//     Responses:
	//       201: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	req := api.ImageConfig{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	if err := req.Valid(); err != nil {
		return http.StatusBadRequest, nil, err
	}

	image, err := ir.bankend.Add(ctx, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, api.TaskObjectResponse{
		//TaskID:     image.Task.ID,
		ObjectID:   image.ID,
		ObjectName: image.String(),
	}, nil
}

// list object options
//
// swagger:parameters listImages,listImageTemplates
type listImageRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Type string `json:"type"`
	// in: query
	// required: false
	Major string `json:"major"`
	// in: query
	// required: false
	Minor string `json:"minor"`
	// in: query
	// required: false
	Patch string `json:"patch"`
	// in: query
	// required: false
	Build string `json:"build"`

	// in: query
	// required: false
	Enabled bool `json:"enabled"`
}

// list images info
//
// swagger:response listImagesResponseWrapper
type listImagesResponseWrapper struct {
	// in: body
	Body api.ImagesResponse
}

func (ir imageRoute) listImages(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/images images listImages
	//
	// 查询镜像信息
	//
	// List Images
	// This will returns a list of images by conditions
	//
	//     Responses:
	//       200: listImagesResponseWrapper
	//       500: ErrorResponse

	opts := api.ImageListOptions{}

	if v := r.FormValue("unschedulable"); v != "" {
		opts.Unschedulable = &v
	}

	if id := r.FormValue("id"); id != "" {
		opts.ID = &id
	}

	if siteID := r.FormValue("site_id"); siteID != "" {
		opts.SiteId = &siteID
	}

	if typ := r.FormValue("type"); typ != "" {
		opts.Type = &typ
	}

	if v := r.FormValue("major"); v != "" {

		opts.Major = &v
	}

	if v := r.FormValue("minor"); v != "" {

		opts.Minor = &v
	}

	if v := r.FormValue("patch"); v != "" {

		opts.Patch = &v
	}

	if val := r.FormValue("build"); val != "" {

		opts.Dev = &val
	}

	list, err := ir.bankend.List(ctx, opts)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if list == nil {
		return http.StatusOK, api.RemoteStoragesResponse{}, nil
	}

	return http.StatusOK, list, nil
}

// update object
//
// swagger:parameters updateImage
type updateImageRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`
	// in: body
	// required: true
	Body api.ImageOptions
}

func (ir imageRoute) updateImage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/images/{id} images updateImage
	//
	// 更改镜像信息
	//
	// Update Images
	// This will update the image
	//
	//     Responses:
	//       200: ObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	req := api.ImageOptions{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	im, err := ir.bankend.Set(ctx, id, req)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, api.ObjectResponse{
		ID:   im.ID,
		Name: im.String(),
	}, nil
}

// object by id
//
// swagger:parameters deleteImage
type deleteImageRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

func (ir imageRoute) deleteImage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route DELETE /manager/images/{id} images deleteImage
	//
	// 删除镜像信息
	//
	// Delete Images
	// This will delete the image by name
	//
	//     Responses:
	//       204: description: OK
	//       500: ErrorResponse

	id := vars["id"]

	out, err := ir.bankend.Delete(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusNoContent, out, nil
}

// list image templates info
//
// swagger:response listImageTemplatesResponseWrapper
type listImageTemplatesResponseWrapper struct {
	// in: body
	Body api.ImageTemplatesResponse
}

func (ir imageRoute) listImageTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/images/templates images listImageTemplates
	//
	// 查询镜像模板信息
	//
	// List Image Templates
	// This will returns a list of image template by conditions
	//
	//     Responses:
	//       200: listImageTemplatesResponseWrapper
	//       500: ErrorResponse

	//opts := api.ImageListOptions{}

	//if v := r.FormValue("enabled"); v != "" {
	//	opts.Enabled = &v
	//}
	//
	//if id := r.FormValue("id"); id != "" {
	//	opts.ID = &id
	//}
	//
	//if typ := r.FormValue("type"); typ != "" {
	//	opts.BackupType = &typ
	//}
	//
	//if v := r.FormValue("major"); v != "" {
	//
	//	opts.Major = &v
	//}
	//
	//if v := r.FormValue("minor"); v != "" {
	//
	//	opts.Minor = &v
	//}
	//
	//if v := r.FormValue("patch"); v != "" {
	//
	//	opts.Patch = &v
	//}
	//
	//if val := r.FormValue("build"); val != "" {
	//
	//	opts.Dev = &val
	//}
	id := vars["id"]

	out, err := ir.bankend.ListImageTemplates(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, err
}

// update object
//
// swagger:parameters updateImageTemplate
type updateImageTemplateRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`
	// in: body
	// required: true
	Body api.ConfigTemplateOptions
}

func (ir imageRoute) updateImageTemplate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/images/{id}/templates images updateImageTemplate
	//
	// 更改镜像模板信息
	//
	// Update Image Template
	// This will update the image template
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	opts := api.ConfigTemplateOptions{}
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	out, err := ir.bankend.SetImageTemplate(ctx, id, opts.Keysets)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

func (ir imageRoute) listImageScripts(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/images/{id}/scripts images listImageScripts
	//
	// 查询镜像脚本内容
	//
	// List Image Script
	// This will list the image script
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse

	id := vars["id"]

	out, err := ir.bankend.ListImageScripts(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, out, nil
}

func (ir imageRoute) syncImageScripts(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route PUT /manager/images/{id}/scripts images syncImageScripts
	//
	// 同步镜像脚本内容
	//
	// Sync Image Script
	// This will sync the image script
	//
	//     Responses:
	//       200: TaskObjectResponse
	//       400: ErrorResponse
	//       500: ErrorResponse
	id := vars["id"]
	scriptType := r.FormValue("type")

	if scriptType == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("type is required")
	}

	err := ir.bankend.SyncImageScripts(ctx, id, scriptType)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}
