package task

import (
	"context"
	"net/http"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterTaskRoute(bankend taskBankend, routers router.Adder) {
	r := &taskRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewGetRoute("/manager/tasks", r.listTasks),
	}

	routers.AddRouter(r)
}

type taskBankend interface {
	List(ctx context.Context, id, relateID, action, state string) ([]api.Task, error)
}

type taskRoute struct {
	bankend taskBankend
	routes  []router.Route
}

func (sr taskRoute) Routes() []router.Route {
	return sr.routes
}

// list object options
//
// swagger:parameters listTasks
type listTaskRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	RelateID string `json:"related_id"`

	// in: query
	// required: false
	Action string `json:"action"`

	// in: query
	// required: false
	Status string `json:"status"`
}

// list tasks info
//
// swagger:response listTasksResponseWrapper
type listTasksResponseWrapper struct {
	// in: body
	Body api.TasksResponse
}

func (sr taskRoute) listTasks(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	// swagger:route GET /manager/tasks tasks listTasks
	//
	// 查询任务列表
	//
	// List Tasks
	// This will returns a list of tasks by options
	//
	//     Responses:
	//       200: listTasksResponseWrapper
	//       500: ErrorResponse

	id := r.FormValue("id")
	relateID := r.FormValue("relate_id")
	action := r.FormValue("action")
	status := r.FormValue("status")

	tasks, err := sr.bankend.List(ctx, id, relateID, action, status)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if tasks == nil {
		return http.StatusOK, api.TasksResponse{}, nil
	}

	return http.StatusOK, tasks, nil
}
