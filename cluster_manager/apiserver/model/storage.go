package model

import (
	"fmt"
	"sync"
)

type RemoteStorage struct {
	Enabled bool   `db:"enabled"`
	ID      string `db:"id"`
	Name    string `db:"name"`
	SiteID  string `db:"site_id"`
	Vendor  string `db:"vendor"`
	Model   string `db:"model"`
	Type    string `db:"type"`
	Desc    string `db:"description"`

	Editor
	Auth

	Site SiteBrief `db:"-"`
	Task Task      `db:"-"`
}

type RemoteStorageBrief struct {
	ID     string `db:"id"`
	Name   string `db:"name"`
	SiteID string `db:"site_id"`
}

func (RemoteStorageBrief) Table() string {
	return RemoteStorage{}.Table()
}

func (rs RemoteStorageBrief) ObjectName() string {
	return rs.ID
}

func (RemoteStorage) Table() string {
	return "tbl_storage_remote"
}

func (rs RemoteStorage) ObjectName() string {
	return rs.ID
}

type Auth struct {
	Port            int    `db:"auth_port"`
	RemoteStorageID string `db:"storage_remote_id"`
	IP              string `db:"auth_ip"`
	User            string `db:"auth_username"`
	Password        string `db:"auth_password"`
	Vstorename      string `db:"auth_vstorename"`
}

func (Auth) Table() string {
	return "tbl_storage_remote_auth"
}

type HostStorage struct {
	MaxUsage    int         `db:"max_usage"`
	ID          string      `db:"id"`
	Host        string      `db:"host_id"`
	Name        string      `db:"name"`
	Performance string      `db:"performance"`
	Paths       SliceString `db:"paths"`
}

func (HostStorage) Table() string {
	return "tbl_storage_host"
}

func NewRemoteStorageModel() ModelRemoteStorage {
	return &modelRemoeteStorage{}
}

type modelRemoeteStorage struct {
	*dbBase
}

func (m *modelRemoeteStorage) Insert(rs RemoteStorage) (string, string, error) {
	if rs.ID == "" {
		rs.ID = newUUID(rs.Name)
	}

	rs.Auth.RemoteStorageID = rs.ID

	task := NewTask(ActionRemoteStorageAdd, rs.ID, rs.Table(), rs.CreatedUser)

	err := m.txFrame(func(tx Tx) error {
		query := "INSERT INTO " + rs.Table() +
			" (id,name,site_id,vendor,model,type,enabled,description,created_user,created_timestamp,modified_user,modified_timestamp) " +
			"VALUES (:id,:name,:site_id,:vendor,:model,:type,:enabled,:description,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

		_, err := tx.NamedExec(query, rs)
		if err != nil {
			return err
		}

		query = "INSERT INTO " + rs.Auth.Table() +
			" (storage_remote_id,auth_ip,auth_port,auth_username,auth_password,auth_vstorename) " +
			"VALUES (:storage_remote_id,:auth_ip,:auth_port,:auth_username,:auth_password,:auth_vstorename)"

		_, err = tx.NamedExec(query, rs.Auth)
		if err != nil {
			return err
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return rs.ID, task.ID, err
}

func (m *modelRemoeteStorage) Update(rs RemoteStorage) (string, error) {
	query := "UPDATE " + rs.Table() +
		" SET name=:name,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	task := NewTask(ActionRemoteStorageEdit, rs.ID, rs.Table(), rs.CreatedUser)

	err := m.txFrame(func(tx Tx) error {
		_, err := tx.NamedExec(query, rs)
		if err != nil {
			return err
		}

		if rs.Auth.RemoteStorageID != "" {

			query := "UPDATE " + rs.Auth.Table() +
				" SET auth_ip=:auth_ip,auth_port=:auth_port,auth_username=:auth_username,auth_password=:auth_password,auth_vstorename=:auth_vstorename " +
				"WHERE storage_remote_id=:storage_remote_id"

			_, err := tx.NamedExec(query, rs.Auth)
			if err != nil {
				return err
			}
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return task.ID, err
}

func (m *modelRemoeteStorage) UpdateRemoteStorageTask(rs *RemoteStorage, tk Task) error {
	if rs == nil {
		return m.UpdateTask(tk)
	}

	query := "UPDATE " + rs.Table() +
		" SET name=:name,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	err := m.txFrame(func(tx Tx) error {
		_, err := tx.NamedExec(query, rs)
		if err != nil {
			return err
		}

		if rs.Auth.RemoteStorageID != "" {

			query := "UPDATE " + rs.Auth.Table() +
				" SET auth_ip=:auth_ip,auth_port=:auth_port,auth_username=:auth_username,auth_password=:auth_password,auth_vstorename=:auth_vstorename" +
				"WHERE storage_remote_id=:storage_remote_id"

			_, err := tx.NamedExec(query, rs.Auth)
			if err != nil {
				return err
			}
		}

		return m.txUpdateTask(tx, tk)
	})

	return err
}

func (m *modelRemoeteStorage) Delete(id string) error {
	query := "DELETE FROM " + RemoteStorage{}.Table() + " WHERE id=?"

	return m.txFrame(func(tx Tx) error {

		_, err := tx.Exec(query, id)
		if err != nil && !IsNotExist(err) {
			return err
		}

		query = "DELETE FROM " + Auth{}.Table() + " WHERE storage_remote_id=?"
		_, err = tx.Exec(query, id)
		if IsNotExist(err) {
			return nil
		}

		return err
	})
}

func (m *dbBase) getRemoteStorageBrief(id string) (RemoteStorageBrief, error) {
	rsb := RemoteStorageBrief{}

	query := "SELECT id,name,site_id FROM " + rsb.Table() + " WHERE id=?"

	err := m.Get(&rsb, query, id)

	return rsb, err
}

func (m *modelRemoeteStorage) Get(id string) (RemoteStorage, error) {

	rs := RemoteStorage{}

	query := "SELECT rs.*,a.* " +
		fmt.Sprintf("FROM %s rs INNER JOIN %s a ON rs.id=a.storage_remote_id", rs.Table(), rs.Auth.Table()) +
		" WHERE rs.id=?"

	err := m.dbBase.Get(&rs, query, id)
	if err != nil {
		return rs, err
	}

	rs.Site, err = m.getSiteBrief(rs.SiteID)
	if err != nil {
		return rs, err
	}

	rs.Task, err = m.latestByRelateID(rs.ID)

	return rs, err
}

func (m *modelRemoeteStorage) List(selector map[string]string) ([]RemoteStorage, error) {

	if id, ok := selector["id"]; ok {

		rs, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []RemoteStorage{rs}, err
	}

	var (
		err error
		arg interface{} = nil

		rs   = RemoteStorage{}
		list = []RemoteStorage{}
	)

	query := "SELECT rs.*,a.storage_remote_id,a.auth_ip,a.auth_port,a.auth_username,a.auth_password,a.auth_vstorename " +
		fmt.Sprintf("FROM %s rs INNER JOIN %s a ON rs.id=a.storage_remote_id", rs.Table(), rs.Auth.Table())

	if name, ok := selector["name"]; ok {

		query = query + " WHERE rs.name=?"
		arg = name

	} else if site, ok := selector["site_id"]; ok {

		query = query + " WHERE rs.site_id=?"
		arg = site

	} else if enabled, ok := selector[labelEnabled]; ok {
		query = query + " WHERE rs.enabled=?"
		arg = enabled
	}

	if arg == nil {
		err = m.Select(&list, query)
	} else {
		err = m.Select(&list, query, arg)
	}
	if err != nil {
		return list, err
	}

	for i := range list {

		list[i].Site, err = m.getSiteBrief(list[i].SiteID)
		if err != nil {
			return list, err
		}

		list[i].Task, err = m.latestByRelateID(list[i].ID)
		if err != nil {
			return list, err
		}
	}

	return list, err
}

type fakeModelRemoeteStorage struct {
	rs    *sync.Map
	pools *sync.Map
	sites *sync.Map
	auths *sync.Map
	tasks ModelTask
}

func (m *fakeModelRemoeteStorage) Insert(rs RemoteStorage) (string, string, error) {
	rs.ID = newUUID(rs.Name)

	rs.Auth.RemoteStorageID = rs.ID

	task := NewTask(ActionRemoteStorageAdd, rs.ID, rs.Table(), rs.CreatedUser)

	m.rs.Store(rs.ID, rs)
	m.auths.Store(rs.Auth.RemoteStorageID, rs.Auth)
	task.ID, _ = m.tasks.Insert(task)

	return rs.ID, task.ID, nil
}

func (m *fakeModelRemoeteStorage) Update(rs RemoteStorage) (string, error) {

	task := NewTask(ActionRemoteStorageEdit, rs.ID, rs.Table(), rs.CreatedUser)

	m.rs.Store(rs.ID, rs)
	m.auths.Store(rs.Auth.RemoteStorageID, rs.Auth)
	task.ID, _ = m.tasks.Insert(task)

	return task.ID, nil
}

func (m *fakeModelRemoeteStorage) UpdateRemoteStorageTask(rs *RemoteStorage, tk Task) error {
	return nil
}

func (m *fakeModelRemoeteStorage) Delete(id string) error {
	v, ok := m.rs.Load(id)
	if !ok {
		return nil
	}

	rs := v.(RemoteStorage)

	m.auths.Range(func(key, value interface{}) bool {

		v, ok := value.(Auth)
		if ok && v.RemoteStorageID == rs.ID {
			rs.Auth = v

			return false
		}

		return true
	})

	m.rs.Delete(rs.ID)
	m.auths.Delete(rs.Auth.RemoteStorageID)

	return nil
}

func (m *fakeModelRemoeteStorage) Get(id string) (RemoteStorage, error) {
	v, ok := m.rs.Load(id)
	if !ok {
		return RemoteStorage{}, NewNotFound("remote storage", id)
	}

	rs := v.(RemoteStorage)

	v, ok = m.sites.Load(rs.SiteID)
	if ok {
		rs.Site.Name = v.(Site).Name
	}

	m.auths.Range(func(key, value interface{}) bool {

		auth, ok := value.(Auth)
		if ok && auth.RemoteStorageID == rs.ID {
			rs.Auth = auth
			return false
		}

		return true
	})

	task, err := m.tasks.LatestByRelateID(rs.ID)
	if err == nil {
		rs.Task = task
	}

	return rs, nil
}
func (m *fakeModelRemoeteStorage) List(selector map[string]string) ([]RemoteStorage, error) {
	if id, ok := selector["id"]; ok {
		rs, err := m.Get(id)

		return []RemoteStorage{rs}, err
	}

	list := make([]RemoteStorage, 0, 2)
	name := selector["name"]
	siteID := selector["site_id"]

	m.rs.Range(func(key, value interface{}) bool {

		v, ok := value.(RemoteStorage)
		if ok &&
			((name != "" && v.Name == name) ||
				(siteID != "" && v.SiteID == siteID)) {

			list = append(list, v)
		}

		return true
	})

	for i := range list {
		v, ok := m.sites.Load(list[i].SiteID)
		if ok {
			list[i].Site.Name = v.(Site).Name
		}

		m.auths.Range(func(key, value interface{}) bool {

			auth, ok := value.(Auth)
			if ok && auth.RemoteStorageID == list[i].ID {
				list[i].Auth = auth

				return false
			}

			return true
		})

		task, err := m.tasks.LatestByRelateID(list[i].ID)
		if err == nil {
			list[i].Task = task
		}
	}

	return list, nil
}

type StoragePool struct {
	Enabled           bool   `db:"enabled"`
	MaxUsage          int    `db:"max_usage"`
	ID                string `db:"id"`
	Name              string `db:"name"`
	Performance       string `db:"performance"`
	Desc              string `db:"description"`
	RemoteStorageID   string `db:"storage_remote_id"`
	RemoteStorageName string `db:"tbl_storage_remote.name"`
	NativeID          string `db:"native_id"`

	Editor
	Task Task `db:"-"`
}

func (StoragePool) Table() string {
	return "tbl_storage_remote_pool"
}

func (ps StoragePool) ObjectName() string {
	return ps.Name
}

func (m *modelRemoeteStorage) InsertPool(pool StoragePool) (string, string, error) {
	pool.ID = newUUID(pool.Name)

	task := NewTask(ActionRemoteStoragePoolAdd, pool.ID, pool.Table(), pool.CreatedUser)

	query := "INSERT INTO " + pool.Table() +
		" (id,storage_remote_id,name,native_id,performance,max_usage,enabled,description,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:storage_remote_id,:name,:native_id,:performance,:max_usage,:enabled,:description,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(query, pool)
		if err != nil {
			return err
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return pool.ID, task.ID, err
}

func (m *modelRemoeteStorage) InsertRemoteStorageTask(rs RemoteStorage, action string) (string, error) {
	tk := NewTask(action, rs.ID, rs.Table(), "")

	return m.insertTask(tk)
}

func (m *modelRemoeteStorage) InsertPoolTask(sp StoragePool, action string) (string, error) {
	tk := NewTask(action, sp.ID, sp.Table(), "")

	return m.insertTask(tk)
}

func (m *modelRemoeteStorage) UpdatePool(pool StoragePool) error {
	query := "UPDATE " + pool.Table() +
		" SET name=:name,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, pool)

	return err
}

func (m *modelRemoeteStorage) UpdatePoolTask(pool *StoragePool, tk Task) error {
	if pool == nil {
		return m.UpdateTask(tk)
	}

	return m.txFrame(func(tx Tx) error {

		query := "UPDATE " + pool.Table() +
			" SET name=:name,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
			"WHERE id=:id"

		_, err := tx.NamedExec(query, pool)
		if err != nil {
			return err
		}

		return m.txUpdateTask(tx, tk)
	})
}

func (m *modelRemoeteStorage) DeletePool(id string) error {
	query := "DELETE FROM " + StoragePool{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *modelRemoeteStorage) GetPool(id string) (StoragePool, error) {
	pool := StoragePool{}
	query := "SELECT * FROM " + pool.Table() + " WHERE id=?"

	err := m.dbBase.Get(&pool, query, id)
	if err != nil {
		return pool, err
	}

	pool.Task, err = m.latestByRelateID(pool.ID)

	return pool, err
}

// TODO: tasks
func (m *modelRemoeteStorage) ListPools(selector map[string]string) ([]StoragePool, error) {
	pool := StoragePool{}
	remote, ok := selector["remote_storage_id"]

	if !ok {
		pools := []StoragePool{}
		query := "SELECT * FROM " + pool.Table()

		err := m.Select(&pools, query)
		if err != nil && !IsNotExist(err) {
			return nil, err
		}

		tasks, err := m.tasksByTable(pool.Table())

		for i := range pools {

			for k := range tasks {
				if pools[i].ID == tasks[k].RelateID &&
					pools[i].Task.Auto < tasks[k].Auto {
					pools[i].Task = tasks[k]
				}
			}
		}

		return pools, nil
	}

	if id, ok := selector["id"]; ok {

		query := "SELECT * FROM " + pool.Table() + " WHERE id=? AND storage_remote_id=?"

		err := m.dbBase.Get(&pool, query, id, remote)
		if IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		pool.Task, err = m.latestByRelateID(pool.ID)

		return []StoragePool{pool}, err
	}

	if name, ok := selector["name"]; ok {

		query := "SELECT * FROM " + pool.Table() + " WHERE name=? AND storage_remote_id=?"

		pools := []StoragePool{}

		err := m.Select(&pools, query, name, remote)

		for i := range pools {
			pools[i].Task, _ = m.latestByRelateID(pools[i].ID)
		}

		return pools, err
	}

	query := "SELECT * FROM " + pool.Table() + " WHERE storage_remote_id=?"

	pools := []StoragePool{}

	err := m.Select(&pools, query, remote)

	tasks, err := m.tasksByTable(pool.Table())

	for i := range pools {

		for k := range tasks {
			if pools[i].ID == tasks[k].RelateID &&
				pools[i].Task.Auto < tasks[k].Auto {
				pools[i].Task = tasks[k]
			}
		}
	}

	return pools, err
}

func (m *fakeModelRemoeteStorage) InsertPool(pool StoragePool) (string, string, error) {
	pool.ID = newUUID(pool.Name)

	task := NewTask(ActionRemoteStoragePoolAdd, pool.ID, pool.Table(), pool.CreatedUser)

	m.pools.Store(pool.ID, pool)
	m.tasks.Insert(task)

	return pool.ID, task.ID, nil
}

func (m *fakeModelRemoeteStorage) InsertRemoteStorageTask(rs RemoteStorage, action string) (string, error) {
	return "", nil
}

func (m *fakeModelRemoeteStorage) InsertPoolTask(sp StoragePool, action string) (string, error) {
	return "", nil
}

func (m *fakeModelRemoeteStorage) UpdatePool(pool StoragePool) error {

	m.pools.Store(pool.ID, pool)

	return nil
}

func (m *fakeModelRemoeteStorage) UpdatePoolTask(pool *StoragePool, tk Task) error {
	return nil
}

func (m *fakeModelRemoeteStorage) DeletePool(id string) error {
	m.pools.Delete(id)

	return nil
}

func (m *fakeModelRemoeteStorage) GetPool(id string) (StoragePool, error) {
	obj, ok := m.pools.Load(id)
	if !ok {
		return StoragePool{}, NewNotFound("remote storage pool", id)
	}

	pool := obj.(StoragePool)

	obj, ok = m.rs.Load(pool.RemoteStorageID)
	if ok {
		pool.RemoteStorageName = obj.(RemoteStorage).Name
	}

	pool.Task, _ = m.tasks.LatestByRelateID(pool.ID)

	return pool, nil
}

func (m *fakeModelRemoeteStorage) ListPools(selector map[string]string) ([]StoragePool, error) {
	if id, ok := selector["id"]; ok {

		pool, err := m.GetPool(id)

		return []StoragePool{pool}, err
	}

	if name, ok := selector["name"]; ok {
		storage := selector["remote_storage_id"]
		storageName := ""

		rs, ok := m.rs.Load(storage)
		if ok {
			storage = rs.(RemoteStorage).ID
			storageName = rs.(RemoteStorage).Name
		}

		pools := make([]StoragePool, 0, 4)

		m.pools.Range(func(key, value interface{}) bool {

			pool, ok := value.(StoragePool)
			if !ok {
				return true
			}

			if pool.Name == name && pool.RemoteStorageID == storage {
				pool.RemoteStorageID = storage
				pool.RemoteStorageName = storageName

				pools = append(pools, pool)
			}

			return true
		})

		for i := range pools {
			pools[i].Task, _ = m.tasks.LatestByRelateID(pools[i].Task.ID)
		}

		return pools, nil
	}

	if storage, ok := selector["remote_storage_id"]; ok {
		name := ""

		obj, ok := m.rs.Load(storage)
		if ok {
			storage = obj.(RemoteStorage).ID
			name = obj.(RemoteStorage).Name
		}

		pools := make([]StoragePool, 0, 4)

		m.pools.Range(func(key, value interface{}) bool {

			pool, ok := value.(StoragePool)
			if !ok {
				return true
			}

			if pool.RemoteStorageID == storage {
				pool.RemoteStorageName = name
				pools = append(pools, pool)
			}

			return true
		})

		for i := range pools {
			pools[i].Task, _ = m.tasks.LatestByRelateID(pools[i].ID)
		}

		return pools, nil
	}

	return nil, nil
}
