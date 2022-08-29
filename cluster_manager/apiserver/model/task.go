package model

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type TaskStatus int

const (
	TaskRunning TaskStatus = iota
	TaskCanceled
	TaskFailed
	TaskSuccess
)

const (
	taskSuccess  = "success"
	taskRunning  = "running"
	taskCanceled = "canceled"
	taskFailed   = "failed"
	taskUnknown  = "unknown"
)

const (
	ActionAppAdd           = "app-add"
	ActionAppDelete        = "app-delete"
	ActionAppImageEdit     = "app-image-edit"
	ActionAppResourceEdit  = "app-resource-edit"
	ActionAppStateEdit     = "app-state-edit"
	ActionAppUnitStateEdit = "app-unit-state-edit"
	ActionAppUnitRebuild   = "app-unit-rebuild"
	ActionAppUnitRestore   = "app-unit-restore"

	ActionHostAdd    = "host-add"
	ActionHostEdit   = "host-edit"
	ActionHostDelete = "host-delete"

	BackupEndpointAdd = "backup-endpoint-add"

	ActionImageAdd          = "image-add"
	ActionImageDelete       = "image-delete"
	ActionImageTemplateEdit = "image-template-edit"

	ActionRemoteStorageAdd        = "remote-storage-add"
	ActionRemoteStorageEdit       = "remote-storage-edit"
	ActionRemoteStorageDelete     = "remote-storage-delete"
	ActionRemoteStoragePoolAdd    = "remote-storage-pool-add"
	ActionRemoteStoragePoolDelete = "remote-storage-pool-delete"
)

func (t Task) String() string {

	if (t.FinishedAt == time.Time{}) {
		t.FinishedAt = time.Now()
	}

	return fmt.Sprintf("ID:%s Act:%s Relate:%s State:%s %s User:%s", t.ID, t.Action, t.RelateID,
		t.Status.State(), t.FinishedAt.Sub(t.CreatedAt), t.User)
}

func (t TaskStatus) State() string {
	switch t {

	case TaskRunning:
		return taskRunning

	case TaskCanceled:
		return taskCanceled

	case TaskFailed:
		return taskFailed

	case TaskSuccess:
		return taskSuccess
	}

	return taskRunning
}

type Task struct {
	// 自增列
	Auto        int    `db:"ai"`
	ID          string `db:"id"`
	Action      string `db:"action"`
	RelateID    string `db:"relate_id"`
	RelateTable string `db:"relate_table"`

	Status TaskStatus `db:"status"`
	Error  string     `db:"error"`

	User       string    `db:"created_user"`
	CreatedAt  time.Time `db:"created_at"`
	FinishedAt time.Time `db:"finished_at"`
}

func (Task) Table() string {
	return "tbl_task"
}

func NewTask(action, relateID, table, user string) Task {
	return Task{
		ID:          taskUUID(relateID),
		Action:      action,
		RelateID:    relateID,
		RelateTable: table,
		User:        user,
		CreatedAt:   time.Now(),
	}
}

func NewTaskModel() ModelTask {
	return &modelTask{}
}

type modelTask struct {
	*dbBase
}

func (db *dbBase) txInsertTask(tx Tx, tk Task) (string, error) {
	query := "INSERT INTO " + tk.Table() +
		" (id,action,relate_id,relate_table,error,status,created_user,created_at,finished_at) " +
		"VALUES (:id,:action,:relate_id,:relate_table,:error,:status,:created_user,:created_at,:finished_at)"

	if tk.ID == "" {
		tk.ID = taskUUID(tk.RelateID)
	}

	_, err := tx.NamedExec(query, tk)

	return tk.ID, err
}

func (m *dbBase) insertTask(tk Task) (string, error) {
	query := "INSERT INTO " + tk.Table() +
		" (id,action,relate_id,relate_table,error,status,created_user,created_at,finished_at) " +
		"VALUES (:id,:action,:relate_id,:relate_table,:error,:status,:created_user,:created_at,:finished_at)"

	if tk.ID == "" {
		tk.ID = taskUUID(tk.RelateID)
	}

	_, err := m.NamedExec(query, tk)

	return tk.ID, err
}

func (m *modelTask) Insert(tk Task) (string, error) {

	return m.insertTask(tk)
}

func (m *modelTask) Update(tk Task) error {
	return m.UpdateTask(tk)
}

func (m *dbBase) UpdateTask(tk Task) error {
	query := "UPDATE " + tk.Table() +
		" SET error=?,status=?,finished_at=? WHERE id=?"

	_, err := m.Exec(query, tk.Error, tk.Status, tk.FinishedAt, tk.ID)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *dbBase) txUpdateTask(tx Tx, tk Task) error {
	query := "UPDATE " + tk.Table() +
		" SET error=?,status=?,finished_at=? WHERE id=?"

	_, err := tx.Exec(query, tk.Error, tk.Status, tk.FinishedAt, tk.ID)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *modelTask) Delete(id string) error {
	query := "DELETE FROM " + Task{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *modelTask) Get(id string) (Task, error) {
	tk := Task{}
	query := "SELECT * FROM " + tk.Table() + " WHERE id=?"

	err := m.dbBase.Get(&tk, query, id)

	return tk, err
}

func (db *dbBase) tasksByTable(table string) ([]Task, error) {
	tasks := []Task{}

	query := "SELECT * FROM " + Task{}.Table() + " WHERE relate_table=?  ORDER BY ai DESC"

	err := db.Select(&tasks, query, table)

	return tasks, err
}

func (db *dbBase) latestByRelateID(id string) (Task, error) {
	tk := Task{}

	query := "SELECT * FROM " + tk.Table() +
		" WHERE relate_id=? ORDER BY ai DESC LIMIT 1"

	err := db.Get(&tk, query, id)
	if IsNotExist(err) {
		return tk, nil
	}

	return tk, err
}

func (m *modelTask) LatestByRelateID(id string) (Task, error) {
	return m.dbBase.latestByRelateID(id)
}

func (m *modelTask) List(selector map[string]string) ([]Task, error) {

	if id, ok := selector["id"]; ok {

		tk, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Task{tk}, err
	}

	if relate, ok := selector["relate_id"]; ok {

		tasks := []Task{}
		query := "SELECT * FROM " + Task{}.Table() +
			" WHERE relate_id=? ORDER BY ai DESC"

		err := m.Select(&tasks, query, relate)

		return tasks, err
	}

	tasks := []Task{}
	query := "SELECT * FROM " + Task{}.Table() +
		" ORDER BY ai DESC"

	err := m.Select(&tasks, query)

	return tasks, err
}

func ReverseTasksByAutoNum(tasks []Task) {

	sort.Slice(tasks,
		func(i, j int) bool {
			return tasks[i].Auto > tasks[j].Auto
		})
}

type fakeModelTask struct {
	tasks *sync.Map
}

func (m *fakeModelTask) Insert(tk Task) (string, error) {

	tk.ID = taskUUID(tk.RelateID)

	m.tasks.Store(tk.ID, tk)

	return tk.ID, nil
}

func (m *fakeModelTask) Update(tk Task) error {
	m.tasks.Store(tk.ID, tk)

	return nil
}

func (m *fakeModelTask) Delete(id string) error {
	m.tasks.Delete(id)
	return nil
}

func (m *fakeModelTask) Get(id string) (Task, error) {
	v, ok := m.tasks.Load(id)
	if !ok {
		return Task{}, NewNotFound("task", id)
	}

	return v.(Task), nil
}

func (m *fakeModelTask) LatestByRelateID(id string) (Task, error) {
	task := Task{}

	m.tasks.Range(func(key, value interface{}) bool {

		tk, ok := value.(Task)
		if ok &&
			tk.RelateID == id &&
			task.Auto < tk.Auto {

			task = tk
		}

		return true
	})

	return task, nil
}

func (m *fakeModelTask) List(selector map[string]string) ([]Task, error) {
	if _, ok := selector["latest"]; ok {

		tasks := make(map[string]Task)
		table := selector["table"]

		m.tasks.Range(func(key, value interface{}) bool {

			tk, ok := value.(Task)
			if !ok || (table != "" && tk.RelateTable != table) {
				return true
			}

			v, ok := tasks[tk.ID]
			if ok && v.Auto < tk.Auto {
				tasks[tk.ID] = tk
			}
			return true
		})

		list := make([]Task, 0, len(tasks))
		for _, tk := range tasks {

			list = append(list, tk)
		}

		return list, nil
	}

	return nil, nil
}
