package model

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	. "github.com/upmio/dbscale-kube/pkg/structs"
	"k8s.io/klog/v2"
	"strings"
	"sync"
	"time"
)

var service2Column = map[string]string{
	MysqlServiceType:    "status_database",
	CmhaServiceType:     "status_cmha",
	ProxysqlServiceType: "status_proxysql",
}

type Application struct {
	Lock           int    `db:"spinlock"`
	ID             string `db:"id"`
	Name           string `db:"name"`
	Desc           string `db:"description"`
	StatusDatabase string `db:"status_database"`
	StatusCmha     string `db:"status_cmha"`
	StatusProxysql string `db:"status_proxysql"`
	Spec           string `db:"spec"`
	SubscriptionId string `db:"subscription_id"`

	Editor

	Units []Unit `db:"-"`
	Task  Task   `db:"-"`
}

func (Application) Table() string {
	return "tbl_app"
}

type Unit struct {
	Lock      int    `db:"spinlock"`
	ID        string `db:"id"`
	Namespace string `db:"namespace"`
	Site      string `db:"site_id"`
	App       string `db:"app_id"`
	Group     string `db:"group_name"`

	Task Task `db:"-"`
}

func (Unit) Table() string {
	return "tbl_unit"
}

func (u Unit) ObjectName() string {
	return u.ID
}

func (u Unit) IsServiceType(serviceType string) bool {
	if serviceType == "" || serviceType == AllServiceType {
		return true
	}

	groups := strings.Split(u.ID, "-")
	if len(groups) < 1 {
		return false
	}

	if serviceType == ProxysqlServiceType {
		serviceType = ProxysqlServiceTypeSN
	}

	return strings.Contains(groups[1], serviceType)
}

func (u Unit) GetServiceType() string {
	groups := strings.Split(u.ID, "-")
	if strings.Contains(groups[1], MysqlServiceType) {
		return MysqlServiceType
	} else if strings.Contains(groups[1], CmhaServiceType) {
		return CmhaServiceType
	} else if strings.Contains(groups[1], ProxysqlServiceType) || strings.Contains(groups[1], ProxysqlServiceTypeSN) {
		return ProxysqlServiceType
	} else {
		klog.Errorf("%s has unknown type", u.ID)
		return "unknown"
	}
}

type JobLevel int

const (
	IdleLevel      JobLevel = iota
	ExclusiveLevel          // 互斥
)

func NewAppModel() ModelApp {
	return &modelApp{}
}

type modelApp struct {
	*dbBase
}

// Insert insert App and Task,returns App.ID and Task.ID
func (m modelApp) Insert(app Application) (string, string, error) {
	if app.ID == "" {
		app.ID = newUUID(app.Name)
	}

	task := NewTask(ActionAppAdd, app.ID, app.Table(), app.CreatedUser)

	query := "INSERT INTO " + app.Table() +
		" (id,name,description,status_database,status_cmha,status_proxysql,spec,subscription_id,spinlock,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:name,:description,:status_database,:status_cmha,:status_proxysql,:spec,:subscription_id,:spinlock,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(query, app)
		if err != nil {
			return err
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return app.ID, task.ID, err
}

func (m modelApp) InsertUnits(units []Unit) error {

	query := "INSERT INTO " + Unit{}.Table() +
		" (id,namespace,site_id,app_id,group_name,spinlock) " +
		"VALUES (:id,:namespace,:site_id,:app_id,:group_name,:spinlock)"

	err := m.txFrame(func(tx Tx) error {

		for i := range units {
			_, err := tx.NamedExec(query, units[i])
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (m modelApp) Update(app Application, action string) (string, error) {

	for i := range app.Units {
		if app.Units[i].ID == "" {
			app.Units[i].ID = newUUID(app.Units[i].Group)
		}

		app.Units[i].App = app.ID
	}

	task := NewTask(action, app.ID, app.Table(), app.ModifiedUser)
	query := "UPDATE " + Application{}.Table() +
		" SET name=:name,description=:description,spec=:spec,spinlock=:spinlock,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(query, app)
		if err != nil {
			return err
		}

		query := "INSERT INTO " + Unit{}.Table() +
			" (id,namespace,site_id,app_id,group_name,spinlock) " +
			"VALUES (:id,:namespace,:site_id,:app_id,:group_name,:spinlock)"

		for i := range app.Units {
			_, err := tx.NamedExec(query, app.Units[i])
			if err != nil {
				return err
			}
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return task.ID, err
}

func (m modelApp) UpdateSpec(app, spec, action, user string, add, remove []Unit) (string, error) {

	for i := range add {
		if add[i].ID == "" {
			add[i].ID = newUUID(add[i].Group)
		}

		add[i].App = app
	}

	task := NewTask(action, app, Application{}.Table(), user)
	query := "UPDATE " + Application{}.Table() +
		" SET spec=?,modified_user=?,modified_timestamp=? " +
		"WHERE id=?"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.Exec(query, spec, user, time.Now(), app)
		if err != nil {
			return err
		}

		query := "INSERT INTO " + Unit{}.Table() +
			" (id,namespace,site_id,app_id,group_name,spinlock) " +
			"VALUES (:id,:namespace,:site_id,:app_id,:group_name,:spinlock)"

		for i := range add {
			_, err := tx.NamedExec(query, add[i])
			if err != nil {
				return err
			}
		}

		query = "DELETE FROM " + Unit{}.Table() + " WHERE id=?"

		for i := range add {
			_, err := tx.Exec(query, remove[i].ID)
			if err != nil {
				return err
			}
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return task.ID, err
}

func (m modelApp) UpdateStatus(app, newStatus, targetService, user string) error {

	col, _ := service2Column[targetService]
	query := "UPDATE " + Application{}.Table() +
		" SET " + col + "=?,modified_user=?,modified_timestamp=? " +
		"WHERE id=?"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.Exec(query, newStatus, user, time.Now(), app)
		return err
	})

	return err
}

func (m modelApp) InsertAppTask(app Application, action string) (string, error) {
	tk := NewTask(action, app.ID, app.Table(), "")

	return m.insertTask(tk)
}

func (m modelApp) InsertUnitTask(unit Unit, action string) (string, error) {
	tk := NewTask(action, unit.ID, unit.Table(), "")

	return m.insertTask(tk)
}

func (m modelApp) UpdateAppTask(app *Application, tk Task) error {
	if app == nil {
		return m.UpdateTask(tk)
	}

	return m.txFrame(func(tx Tx) error {
		query := "UPDATE " + Application{}.Table() +
			" SET name=:name,description=:description,spec=:spec,spinlock=:spinlock,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
			"WHERE id=:id"

		_, err := tx.NamedExec(query, app)
		if err != nil {
			return err
		}

		return m.txUpdateTask(tx, tk)
	})
}

func (m modelApp) Delete(id string) error {
	query := "DELETE FROM " + Application{}.Table() + " WHERE id=?"

	return m.txFrame(func(tx Tx) error {

		_, err := tx.Exec(query, id)
		if err != nil && !IsNotExist(err) {
			return err
		}

		query := "DELETE FROM " + Unit{}.Table() + " WHERE app_id=?"

		_, err = tx.Exec(query, id)
		if IsNotExist(err) {
			return nil
		}

		return err
	})
}

func (m modelApp) Get(id string) (Application, error) {
	app := Application{}
	query := "SELECT * FROM " + app.Table() + " WHERE id=?"

	err := m.dbBase.Get(&app, query, id)
	if err != nil {
		return app, err
	}

	query = "SELECT * FROM " + Unit{}.Table() + " WHERE app_id=?"
	err = m.Select(&app.Units, query, app.ID)
	if err != nil {
		return app, err
	}

	app.Task, err = m.latestByRelateID(app.ID)

	return app, err
}

func (m modelApp) GetWithSubscriptionId(id, subscriptionId string) (Application, error) {
	var err error
	app := Application{}
	query := "SELECT * FROM " + app.Table() + " WHERE id=?"

	if subscriptionId != "" {
		query := "SELECT * FROM " + app.Table() + " WHERE id=? and subscription_id=?"
		err = m.dbBase.Get(&app, query, id, subscriptionId)
	} else {
		query := "SELECT * FROM " + app.Table() + " WHERE id=?"
		err = m.dbBase.Get(&app, query, id)
	}
	if err != nil {
		return app, err
	}

	query = "SELECT * FROM " + Unit{}.Table() + " WHERE app_id=?"
	err = m.Select(&app.Units, query, app.ID)
	if err != nil {
		return app, err
	}

	app.Task, err = m.latestByRelateID(app.ID)

	return app, err
}

func (m modelApp) List(selector map[string]string) ([]Application, error) {
	//get single app by id
	if id, ok := selector["id"]; ok {

		subscriptionId, _ := selector["subscription_id"]
		app, err := m.GetWithSubscriptionId(id, subscriptionId)
		if IsNotExist(err) {
			return nil, nil
		}

		for i := range app.Units {
			app.Units[i].Task, _ = m.latestByRelateID(app.Units[i].ID)
		}

		return []Application{app}, err
	}

	//get apps by name
	var apps []Application

	if name, ok := selector["name"]; ok {
		var err error

		subscriptionId, ok := selector["subscription_id"]
		if !ok {
			query := "SELECT * FROM " + Application{}.Table() + " WHERE name=?"
			err = m.dbBase.Select(&apps, query, name)
			if err != nil {
				return apps, err
			}
		} else {
			query := "SELECT * FROM " + Application{}.Table() + " WHERE name=? and subscription_id=?"
			err = m.dbBase.Select(&apps, query, name, subscriptionId)
			if err != nil {
				return apps, err
			}
		}

		for i := range apps {

			query := "SELECT * FROM " + Unit{}.Table() + " WHERE app_id=?"
			err = m.Select(&apps[i].Units, query, apps[i].ID)
			if err != nil {
				return apps, err
			}

			apps[i].Task, _ = m.latestByRelateID(apps[i].ID)

			for k, u := range apps[i].Units {
				u.Task, _ = m.latestByRelateID(u.ID)

				apps[i].Units[k] = u
			}
		}

		return apps, err
	}

	var err error
	subscriptionId, ok := selector["subscription_id"]
	if !ok {
		query := "SELECT * FROM " + Application{}.Table()
		err = m.dbBase.Select(&apps, query)
	} else {
		query := "SELECT * FROM " + Application{}.Table() + " where subscription_id=?"
		err = m.dbBase.Select(&apps, query, subscriptionId)
	}
	if err != nil {
		return apps, err
	}

	var units []Unit

	query := "SELECT * FROM " + Unit{}.Table()
	err = m.Select(&units, query)
	if err != nil {
		return apps, err
	}

	tasks, err := m.tasksByTable(Application{}.Table())
	if err != nil {
		return apps, err
	}

	utasks, err := m.tasksByTable(Unit{}.Table())
	if err != nil {
		return apps, err
	}

	for i := range apps {

		for k := range units {

			if units[k].Task.ID == "" {

				for l := range utasks {
					if units[k].ID == utasks[l].RelateID &&
						units[k].Task.Auto < utasks[l].Auto {

						units[k].Task = utasks[l]
					}
				}
			}

			if apps[i].ID == units[k].App {
				apps[i].Units = append(apps[i].Units, units[k])
			}
		}

		for l := range tasks {
			if apps[i].ID == tasks[l].RelateID &&
				apps[i].Task.Auto < tasks[l].Auto {
				apps[i].Task = tasks[l]
			}
		}
	}

	return apps, nil
}

func (m modelApp) Count(selector map[string]string) (uint, error) {
	var count []uint
	var err error
	var whereClause, name, subscriptionId = AppSelector2sql(selector)
	if whereClause == "" {
		query := "SELECT count(*) FROM " + Application{}.Table()
		err = m.dbBase.Select(&count, query)
	} else {
		query := "SELECT count(*) FROM " + Application{}.Table() + " WHERE " + whereClause
		if name != "" && subscriptionId != "" {
			err = m.dbBase.Select(&count, query, name, subscriptionId)
		}
		if name != "" && subscriptionId == "" {
			err = m.dbBase.Select(&count, query, name)
		}
		if name == "" && subscriptionId != "" {
			err = m.dbBase.Select(&count, query, subscriptionId)
		}
	}

	if err != nil {
		return 0, err
	}
	return count[0], nil
}

func AppSelector2sql(selector map[string]string) (string, string, string) {
	var ret string
	name, ok := selector["name"]
	if ok {
		ret += fmt.Sprintf(" name = ? ")
	}
	subscriptionId, ok := selector["subscription_id"]
	if ok {
		if ret != "" {
			ret += " AND "
		}
		ret += fmt.Sprintf(" subscription_id = ? ")
	}
	return ret, name, subscriptionId
}

func (m modelApp) ListWithPagination(selector map[string]string, pagination api.PaginationReq) ([]Application, api.PaginationResp, error) {
	//get single app by id
	if id, ok := selector["id"]; ok {

		subscriptionId, _ := selector["subscription_id"]
		app, err := m.GetWithSubscriptionId(id, subscriptionId)
		if IsNotExist(err) {
			return nil, api.NewPaginationRespError("not found"), nil
		}

		for i := range app.Units {
			app.Units[i].Task, _ = m.latestByRelateID(app.Units[i].ID)
		}

		return []Application{app}, api.NewPaginationRespSingle(true, nil), err
	}

	//get apps by name
	var apps []Application
	var err error
	total, err := m.Count(selector)
	if err != nil {
		return nil, api.NewPaginationRespError("sql error"), err
	}

	var whereClause, name, subscriptionId = AppSelector2sql(selector)

	if whereClause == "" {
		query := "SELECT * FROM " + Application{}.Table() + api.PaginationToSql("id", pagination)
		err = m.dbBase.Select(&apps, query)
	} else {
		query := "SELECT * FROM " + Application{}.Table() + " WHERE " + whereClause + api.PaginationToSql("id", pagination)
		if name != "" && subscriptionId != "" {
			err = m.dbBase.Select(&apps, query, name, subscriptionId)
		}
		if name != "" && subscriptionId == "" {
			err = m.dbBase.Select(&apps, query, name)
		}
		if name == "" && subscriptionId != "" {
			err = m.dbBase.Select(&apps, query, subscriptionId)
		}
	}

	if err != nil {
		return apps, api.NewPaginationRespError("sql error"), err
	}

	if len(apps) < 1 {
		return apps, api.NewPaginationRespOk(pagination, 0, nil), nil
	}

	var inClause []string
	for _, app := range apps {
		inClause = append(inClause, app.ID)
	}

	var units []Unit

	query, args, err := sqlx.In("SELECT * FROM "+Unit{}.Table()+" WHERE app_id IN (?)", inClause)
	if err != nil {
		return apps, api.NewPaginationRespError("sql error"), err
	}

	query = m.Rebind(query)
	err = m.Select(&units, query, args...)
	if err != nil {
		return apps, api.NewPaginationRespError("sql error"), err
	}

	tasks, err := m.tasksByTable(Application{}.Table())
	if err != nil {
		return apps, api.NewPaginationRespError("sql error"), err
	}

	utasks, err := m.tasksByTable(Unit{}.Table())
	if err != nil {
		return apps, api.NewPaginationRespError("sql error"), err
	}

	for i := range apps {

		for k := range units {

			if units[k].Task.ID == "" {

				for l := range utasks {
					if units[k].ID == utasks[l].RelateID &&
						units[k].Task.Auto < utasks[l].Auto {

						units[k].Task = utasks[l]
					}
				}
			}

			if apps[i].ID == units[k].App {
				apps[i].Units = append(apps[i].Units, units[k])
			}
		}

		for l := range tasks {
			if apps[i].ID == tasks[l].RelateID &&
				apps[i].Task.Auto < tasks[l].Auto {
				apps[i].Task = tasks[l]
			}
		}
	}

	return apps, api.NewPaginationRespOk(pagination, total, apps), err
}

type fakeModelApp struct {
	apps  *sync.Map
	units *sync.Map
	hosts ModelHost
	tasks ModelTask
}

// Insert insert App and Task,returns App.ID and Task.ID
func (m fakeModelApp) Insert(app Application) (string, string, error) {
	app.ID = newUUID(app.Name)

	task := NewTask(ActionAppAdd, app.ID, app.Table(), app.CreatedUser)

	m.apps.Store(app.ID, app)

	for i := range app.Units {
		app.Units[i].ID = newUUID(app.Units[i].Group)

		m.units.Store(app.Units[i].ID, app.Units[i])
	}

	task.ID, _ = m.tasks.Insert(task)

	return app.ID, task.ID, nil
}

func (m fakeModelApp) InsertUnits(units []Unit) error {
	return nil
}

func (m fakeModelApp) UpdateSpec(app, spec, action, user string, add, remove []Unit) (string, error) {
	v, ok := m.apps.Load(app)
	if !ok {
		return "", NewNotFound("app", app)
	}

	a := v.(Application)
	a.Spec = spec

	units := make([]Unit, 0, len(a.Units))
	for i := range a.Units {

		exist := false

		for k := range remove {
			if a.Units[i].ID == remove[k].ID {
				exist = true
				break
			}
		}

		if !exist {
			units = append(units, a.Units[i])
		}
	}

	a.Units = units
	if len(add) > 0 {
		a.Units = append(a.Units, add...)
	}

	task := NewTask(action, app, Application{}.Table(), user)
	task.ID, _ = m.tasks.Insert(task)

	return task.ID, nil
}

func (m fakeModelApp) Update(app Application, action string) (string, error) {
	m.apps.Store(app.ID, app)

	for i := range app.Units {
		if app.Units[i].ID == "" {
			app.Units[i].ID = newUUID(app.Units[i].Group)
		}

		m.units.Store(app.Units[i].ID, app.Units[i])
	}

	task := NewTask(action, app.ID, app.Table(), app.ModifiedUser)
	task.ID, _ = m.tasks.Insert(task)

	return task.ID, nil
}

func (m fakeModelApp) UpdateStatus(app, newStatus, targetService, user string) error {
	return nil
}

func (m fakeModelApp) UpdateAppTask(app *Application, tk Task) error {
	return nil
}

func (m fakeModelApp) InsertAppTask(app Application, action string) (string, error) {
	return "", nil
}

func (m fakeModelApp) InsertUnitTask(unit Unit, action string) (string, error) {
	return "", nil
}

func (m fakeModelApp) Delete(id string) error {
	units := make([]Unit, 0, 2)

	m.units.Range(func(key, value interface{}) bool {
		u, ok := value.(Unit)
		if ok && u.App == id {
			units = append(units, u)
		}

		return true
	})

	m.apps.Delete(id)
	for i := range units {
		m.units.Delete(units[i].ID)
	}

	return nil
}
func (m fakeModelApp) Get(id string) (Application, error) {
	v, ok := m.apps.Load(id)
	if !ok {
		return Application{}, NewNotFound("app", id)
	}

	app := v.(Application)
	units := make([]Unit, 0, len(app.Units))

	m.units.Range(func(key, value interface{}) bool {

		u, ok := value.(Unit)
		if ok && u.App == app.ID {
			units = append(units, u)
		}

		return true
	})

	app.Task, _ = m.tasks.LatestByRelateID(app.ID)

	return app, nil
}

func (m fakeModelApp) List(selector map[string]string) ([]Application, error) {
	if id, ok := selector["id"]; ok {

		app, err := m.Get(id)

		return []Application{app}, err
	}

	apps := make([]Application, 0, 10)

	if name, ok := selector["name"]; ok {

		m.apps.Range(func(key, value interface{}) bool {

			app, ok := value.(Application)
			if ok && app.Name == name {
				apps = append(apps, app)
			}

			return true
		})

	} else {
		m.apps.Range(func(key, value interface{}) bool {

			app, ok := value.(Application)
			if ok {
				apps = append(apps, app)
			}

			return true
		})
	}

	for i := range apps {

		units := make([]Unit, 0, 2)

		m.units.Range(func(key, value interface{}) bool {

			u, ok := value.(Unit)
			if ok && u.App == apps[i].ID {
				units = append(units, u)
			}

			return true
		})

		apps[i].Units = units

		apps[i].Task, _ = m.tasks.LatestByRelateID(apps[i].ID)
	}

	return apps, nil
}

func (m fakeModelApp) ListWithPagination(selector map[string]string, pagination api.PaginationReq) ([]Application, api.PaginationResp, error) {
	return nil, api.NewPaginationRespError("test"), nil
}
