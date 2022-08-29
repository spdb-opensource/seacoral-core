package model

import (
	"errors"
	"sync"

	"github.com/jmoiron/sqlx"
)

type Host struct {
	Enabled bool   `db:"enabled"`
	ID      string `db:"id"`
	// Name              string `db:"name"`
	Hostname        string `db:"host_name"`
	IP              string `db:"host_ip"`
	ClusterID       string `db:"cluster_id"`
	RemoteStorageID string `db:"storage_remote_id"`

	Desc string `db:"description"`
	Location
	Max
	Editor

	RemoteStorageName string        `db:"-"`
	Cluster           ClusterBrief  `db:"-"`
	HostStorages      []HostStorage `db:"-"`
	Task              Task          `db:"-"`
}

type HostBrief struct {
	ID        string       `db:"id"`
	HostName  string       `db:"host_name"`
	IP        string       `db:"host_ip"`
	ClusterID string       `db:"cluster_id"`
	Cluster   ClusterBrief `db:"-"`
}

func (HostBrief) Table() string {
	return Host{}.Table()
}

func (h HostBrief) ObjectName() string {
	return h.IP
}

type Max struct {
	MaxUnit              int `db:"unit_max"`
	MaxUsageCPU          int `db:"max_usage_cpu"`
	MaxUsageMemory       int `db:"max_usage_mem"`
	MaxUsageNetBandwidth int `db:"max_usage_bandwidth"`
}

type Location struct {
	// 机房
	Room string `db:"room"`
	// 位置
	Seat string `db:"seat"`
}

func (Host) Table() string {
	return "tbl_host"
}

func (h Host) ObjectName() string {
	return h.IP
}

func NewHostModel() ModelHost {
	return &modelHost{}
}

type modelHost struct {
	*dbBase
}

func (m *modelHost) Insert(h Host) (string, string, error) {
	if h.ID == "" {
		h.ID = newUUID("")
	}

	task := NewTask(ActionHostAdd, h.ID, h.Table(), h.CreatedUser)

	qh := "INSERT INTO " + h.Table() +
		" (id,host_name,host_ip,cluster_id,room,seat,storage_remote_id,max_usage_cpu,max_usage_mem,max_usage_bandwidth,unit_max,enabled,description,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:host_name,:host_ip,:cluster_id,:room,:seat,:storage_remote_id,:max_usage_cpu,:max_usage_mem,:max_usage_bandwidth,:unit_max,:enabled,:description,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	qsh := "INSERT INTO " + HostStorage{}.Table() +
		" (id,host_id,name,performance,paths,max_usage) " +
		"VALUES (:id,:host_id,:name,:performance,:paths,:max_usage)"

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(qh, h)
		if err != nil {
			return err
		}

		for i := range h.HostStorages {
			h.HostStorages[i].ID = newUUID(h.HostStorages[i].Name)
			h.HostStorages[i].Host = h.ID

			_, err := tx.NamedExec(qsh, h.HostStorages[i])
			if err != nil {
				return err
			}
		}

		task.ID, err = m.txInsertTask(tx, task)

		return err
	})

	return h.ID, task.ID, err
}

func (m *modelHost) InsertHostTask(h Host, action string) (string, error) {
	tk := NewTask(action, h.ID, h.Table(), "")

	return m.insertTask(tk)
}

func (m *modelHost) UpdateHostTask(h *Host, tk Task) error {
	if h == nil {
		return m.UpdateTask(tk)
	}

	return m.txFrame(func(tx Tx) error {

		query := "UPDATE " + h.Table() +
			" SET room=:room,seat=:seat,max_usage_cpu=:max_usage_cpu,max_usage_mem=:max_usage_mem,max_usage_bandwidth=:max_usage_bandwidth,unit_max=:unit_max," +
			"enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
			"WHERE id=:id"

		_, err := tx.NamedExec(query, h)
		if err != nil {
			return err
		}

		return m.txUpdateTask(tx, tk)
	})
}

func (m *modelHost) Update(h Host, storageMaxUsage *int) (string, error) {
	query := "UPDATE " + h.Table() +
		" SET room=:room,seat=:seat,max_usage_cpu=:max_usage_cpu,max_usage_mem=:max_usage_mem,max_usage_bandwidth=:max_usage_bandwidth,unit_max=:unit_max," +
		"enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	tk := NewTask(ActionHostEdit, h.ID, h.Table(), h.ModifiedUser)

	err := m.txFrame(func(tx Tx) error {

		_, err := tx.NamedExec(query, h)
		if err != nil {
			return err
		}

		if storageMaxUsage != nil {

			query := "UPDATE " + HostStorage{}.Table() + " SET max_usage=? " +
				"WHERE host_id=?"

			_, err = tx.Exec(query, *storageMaxUsage, h.ID)
			if err != nil {
				return err
			}
		}

		tk.ID, err = m.txInsertTask(tx, tk)

		return err
	})

	return tk.ID, err
}

func (m *modelHost) Delete(id string) error {

	query := "DELETE FROM " + Host{}.Table() + " WHERE id=?"

	return m.txFrame(func(tx Tx) error {

		_, err := tx.Exec(query, id)
		if err != nil && !IsNotExist(err) {
			return err
		}

		query = "DELETE FROM " + HostStorage{}.Table() + " WHERE host_id=?"

		_, err = tx.Exec(query, id)
		if IsNotExist(err) {
			return nil
		}

		return err
	})
}

func (m *modelHost) GetHostBrief(id string) (HostBrief, error) {
	h := HostBrief{}

	query := "SELECT id,host_name,host_ip,cluster_id FROM " + h.Table() + " WHERE id=? OR host_name=? OR host_ip=?"

	err := m.dbBase.Get(&h, query, id, id, id)
	if err != nil {
		return h, err
	}

	h.Cluster, err = m.getClusterBrief(h.ClusterID)

	return h, err
}

func (m *modelHost) Get(id string) (Host, error) {
	h := Host{}
	query := "SELECT * FROM " + h.Table() + " WHERE id=?"

	err := m.dbBase.Get(&h, query, id)
	if err != nil {
		return h, err
	}

	h.Cluster, err = m.getClusterBrief(h.ClusterID)
	if err != nil {
		return h, err
	}

	query = "SELECT * FROM " + HostStorage{}.Table() +
		" WHERE host_id=?"
	err = m.Select(&h.HostStorages, query, h.ID)
	if err != nil {
		return h, err
	}

	if h.RemoteStorageID != "" {
		rsb, err := m.getRemoteStorageBrief(h.RemoteStorageID)
		if err != nil {
			return h, err
		}

		h.RemoteStorageName = rsb.Name
	}

	h.Task, err = m.latestByRelateID(h.ID)

	return h, err
}

func (m *modelHost) List(selector map[string]string) ([]Host, error) {
	if id, ok := selector["id"]; ok {

		host, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Host{host}, err
	}

	var (
		args     []interface{}
		hosts    []Host
		clusters = []ClusterBrief{}
		query    = "SELECT * FROM " + Host{}.Table()
	)

	if cluster, ok := selector["cluster_id"]; ok {
		c, err := m.getClusterBrief(cluster)
		if err != nil {
			return nil, err
		}

		clusters = []ClusterBrief{c}

		args = append(args, cluster)
		query = query + " WHERE cluster_id=?"

		if enable, ok := selector[labelEnabled]; ok {
			query = query + " AND enabled=?"
			args = append(args, enable)
		}

	} else if site, ok := selector["site_id"]; ok {

		q := "SELECT id,name site_id FROM " + Cluster{}.Table() +
			" WHERE site_id=?"

		err := m.Select(&clusters, q, site)
		if err != nil {
			return nil, err
		}

		args = make([]interface{}, 0, len(clusters))
		for i := range clusters {
			args = append(args, clusters[i].ID)
		}

		query = query + " WHERE cluster_id IN (?)"

		query, args, err = sqlx.In(query, args)
		if err != nil {
			return hosts, err
		}

		err = m.Select(&hosts, query, args...)
		if err != nil {
			return hosts, err
		}

		if enable, ok := selector[labelEnabled]; ok {
			v := enable == "1"
			list := make([]Host, 0, len(hosts))

			for i := range hosts {
				if hosts[i].Enabled == v {
					list = append(list, hosts[i])
				}
			}

			hosts = list
		}

		query = ""
	} else if enable, ok := selector[labelEnabled]; ok {
		query = query + " WHERE enabled=?"
		args = append(args, enable)
	}

	if query != "" {
		err := m.Select(&hosts, query, args...)
		if err != nil {
			return hosts, err
		}
	}

	if len(clusters) > 0 {
		for i := range hosts {
			for k := range clusters {
				if hosts[i].ClusterID == clusters[k].ID {
					hosts[i].Cluster = clusters[k]
					break
				}
			}
		}

	} else {
		for i := range hosts {
			hosts[i].Cluster, _ = m.getClusterBrief(hosts[i].ClusterID)
		}
	}

	rss := make(map[string]string)

	query = "SELECT * FROM " + HostStorage{}.Table() +
		" WHERE host_id=?"
	for i := range hosts {
		err := m.Select(&hosts[i].HostStorages, query, hosts[i].ID)
		if err != nil {
			return hosts, err
		}

		if hosts[i].RemoteStorageID != "" {
			name, ok := rss[hosts[i].RemoteStorageID]
			if ok {
				hosts[i].RemoteStorageName = name
			} else {

				rsb, err := m.getRemoteStorageBrief(hosts[i].RemoteStorageID)
				if err != nil {
					return hosts, err
				}

				hosts[i].RemoteStorageName = rsb.Name
				rss[hosts[i].RemoteStorageID] = rsb.Name
			}
		}
	}

	for i := range hosts {
		hosts[i].Task, _ = m.latestByRelateID(hosts[i].ID)
	}

	return hosts, nil
}

type fakeModelHost struct {
	hosts    *sync.Map
	clusters *sync.Map
	hss      *sync.Map // host storages
	rss      *sync.Map // remote storages
	tasks    ModelTask
}

func (m *fakeModelHost) Insert(h Host) (string, string, error) {
	h.ID = newUUID("")

	task := NewTask(ActionHostAdd, h.ID, h.Table(), h.CreatedUser)

	m.hosts.Store(h.ID, h)
	for i := range h.HostStorages {
		h.HostStorages[i].ID = newUUID(h.HostStorages[i].Name)
		h.HostStorages[i].Host = h.ID
		m.hss.Store(h.HostStorages[i].ID, h.HostStorages[i])
	}
	task.ID, _ = m.tasks.Insert(task)

	return h.ID, task.ID, nil
}

func (m *fakeModelHost) InsertHostTask(h Host, action string) (string, error) {
	return "", nil
}

func (m *fakeModelHost) UpdateHostTask(h *Host, tk Task) error {
	return nil
}

func (m *fakeModelHost) Update(h Host, storageMaxUsage *int) (string, error) {
	if h.ID == "" {
		return "", errors.New("id is required")
	}

	m.hosts.Store(h.ID, h)

	if storageMaxUsage != nil {
		for i := range h.HostStorages {
			m.hss.Store(h.HostStorages[i].ID, h.HostStorages)
		}
	}

	tk := NewTask(ActionHostEdit, h.ID, h.Table(), h.ModifiedUser)
	tk.ID, _ = m.tasks.Insert(tk)

	return tk.ID, nil
}

func (m *fakeModelHost) Delete(id string) error {
	val, ok := m.hosts.Load(id)
	if !ok {
		return nil
	}

	host := val.(Host)
	hss := make([]HostStorage, 0, 2)

	m.hss.Range(func(key, value interface{}) bool {

		v, ok := value.(HostStorage)
		if ok && v.Host == host.ID {
			hss = append(hss, v)
		}

		return true
	})

	m.hosts.Delete(host.ID)

	for i := range hss {
		m.hss.Delete(hss[i].ID)
	}

	return nil
}

func (m *fakeModelHost) GetHostBrief(id string) (HostBrief, error) {
	return HostBrief{}, nil
}
func (m *fakeModelHost) Get(id string) (Host, error) {

	v, ok := m.hosts.Load(id)
	if !ok {
		return Host{}, NewNotFound("host", id)
	}

	host := v.(Host)

	v, ok = m.clusters.Load(host.ClusterID)
	if ok {
		c := v.(Cluster)

		host.Cluster.Name = c.Name
		host.Cluster.SiteID = c.SiteID
	}

	hss := make([]HostStorage, 0, 2)

	m.hss.Range(func(key, value interface{}) bool {

		v, ok := value.(HostStorage)
		if ok && v.Host == host.ID {

			hss = append(hss, v)
		}

		return true
	})

	host.HostStorages = hss

	v, ok = m.rss.Load(host.RemoteStorageID)
	if ok {
		host.RemoteStorageName = v.(RemoteStorage).Name
	}

	task, err := m.tasks.LatestByRelateID(host.ID)
	if err == nil {
		host.Task = task
	}

	return host, nil
}

func (m *fakeModelHost) List(selector map[string]string) ([]Host, error) {
	if id, ok := selector["id"]; ok {
		host, err := m.Get(id)

		return []Host{host}, err
	}

	hosts := make([]Host, 0, 5)

	clusters := make([]Cluster, 0, 2)
	clusterID := selector["cluster_id"]
	siteID := selector["site_id"]

	m.clusters.Range(func(key, value interface{}) bool {

		v, ok := value.(Cluster)

		if ok && (len(selector) == 0 ||
			(clusterID != "" && v.ID == clusterID) ||
			(siteID != "" && v.SiteID == siteID)) {

			clusters = append(clusters, v)
		}

		return true
	})

	m.hosts.Range(func(key, value interface{}) bool {

		v, ok := value.(Host)
		if !ok {
			return true
		}

		for i := range clusters {

			if v.ClusterID == clusters[i].ID {
				v.Cluster.Name = clusters[i].Name
				v.Cluster.SiteID = clusters[i].SiteID

				hosts = append(hosts, v)
				break
			}
		}

		return true
	})

	for i := range hosts {

		hss := make([]HostStorage, 0, 2)

		m.hss.Range(func(key, value interface{}) bool {

			v, ok := value.(HostStorage)
			if ok && v.Host == hosts[i].ID {

				hss = append(hss, v)
			}

			return true
		})

		hosts[i].HostStorages = hss

		v, ok := m.rss.Load(hosts[i].RemoteStorageID)
		if ok {
			hosts[i].RemoteStorageName = v.(RemoteStorage).Name
		}

		hosts[i].Task, _ = m.tasks.LatestByRelateID(hosts[i].ID)
	}

	return hosts, nil
}

func (m *fakeModelHost) ListUnits() ([]Unit, error) {
	return []Unit{}, nil
}

func (m *modelHost) ListUnits() ([]Unit, error) {
	var units []Unit

	query := "SELECT * FROM " + Unit{}.Table()
	err := m.Select(&units, query)
	if err != nil {
		if IsNotExist(err) {
			return units, nil
		}
		return units, err
	}

	return units, nil
}
