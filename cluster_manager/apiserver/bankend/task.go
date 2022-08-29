package bankend

import (
	"context"
	"strings"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
)

func NewTaskBankend(getter taskGetter) *bankendTask {
	return &bankendTask{
		getter: getter,
	}
}

type modelTask interface {
	taskGetter
}

type taskGetter interface {
	Get(name string) (model.Task, error)
	List(selector map[string]string) ([]model.Task, error)
}

type bankendTask struct {
	getter taskGetter
}

func (b *bankendTask) List(ctx context.Context, id, relateID, action, state string) ([]api.Task, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}
	if relateID != "" {
		selector["related_id"] = relateID
	}
	if action != "" {
		selector["action"] = action
	}
	if state != "" {
		selector["state"] = state
	}

	list, err := b.getter.List(selector)
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	model.ReverseTasksByAutoNum(list)

	tasks := make([]api.Task, len(list))

	for i := range list {
		tasks[i] = convertToTask(list[i])
	}

	return tasks, nil
}

func convertToTask(task model.Task) api.Task {
	return api.Task{
		ID:         task.ID,
		User:       task.User,
		Action:     task.Action,
		RelateID:   task.RelateID,
		Error:      task.Error,
		Status:     task.Status.State(),
		CreatedAt:  api.Time(task.CreatedAt),
		FinishedAt: api.Time(task.FinishedAt),
	}
}

func convertToTaskBrief(task model.Task) api.TaskBrief {
	return api.TaskBrief{
		ID:     task.ID,
		Status: task.Status.State(),
		Action: task.Action,
		User:   task.User,
	}
}

func taskUpdate(id string, err error) model.Task {
	tk := model.Task{
		ID:         id,
		Status:     model.TaskSuccess,
		FinishedAt: time.Now(),
	}

	if err != nil {
		tk.Error = err.Error()
		tk.Status = model.TaskFailed
	}

	return tk
}

func boolValue(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))

	if s == "1" || s == "yes" || s == "true" {
		return "1"
	}

	return "0"
}
