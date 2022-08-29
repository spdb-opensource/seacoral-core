package model

import (
	"errors"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

type ClusterBrief struct {
	ID     string `db:"id"`
	Name   string `db:"name"`
	SiteID string `db:"site_id"`
}

func (ClusterBrief) Table() string {
	return Cluster{}.Table()
}

type Cluster struct {
	Enabled bool        `db:"enabled"`
	ID      string      `db:"id"`
	Name    string      `db:"name"`
	SiteID  string      `db:"site_id"`
	Zone    string      `db:"zone"`
	HaTag   string      `db:"ha_tag"`
	Desc    string      `db:"description"`
	Images  SliceString `db:"image_type"`
	Editor
	Site SiteBrief `db:"-"`
}

type Editor struct {
	CreatedUser  string    `db:"created_user"`
	ModifiedUser string    `db:"modified_user"`
	CreatedAt    time.Time `db:"created_timestamp,omitempty"`
	ModifiedAt   time.Time `db:"modified_timestamp,omitempty"`
}

func (Cluster) Table() string {
	return "tbl_cluster"
}

func NewClusterModel() ModelCluster {
	return &modelCluster{}
}

type modelCluster struct {
	*dbBase
}

// Insert insert Cluster,returns Cluster.ID
func (m *modelCluster) Insert(c Cluster) (string, error) {
	query := "INSERT INTO " + c.Table() +
		" (id,name,site_id,zone,image_type,ha_tag,enabled,description,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:name,:site_id,:zone,:image_type,:ha_tag,:enabled,:description,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	if c.ID == "" {
		c.ID = newUUID(c.Name)
	}

	_, err := m.NamedExec(query, c)

	return c.ID, err
}

func (m *modelCluster) Update(c Cluster) error {
	query := "UPDATE " + c.Table() +
		" SET name=:name,zone=:zone,image_type=:image_type,ha_tag=:ha_tag," +
		"enabled=:enabled,description=:description," +
		"modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, c)

	return err
}

func (m *modelCluster) Delete(id string) error {
	query := "DELETE FROM " + Cluster{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (db *dbBase) getClusterBrief(id string) (ClusterBrief, error) {

	c := ClusterBrief{}
	query := "SELECT id,name,site_id FROM " + c.Table() + " WHERE id=?"

	if id == "" {
		return c, nil
	}

	err := db.Get(&c, query, id)

	return c, err
}

func (m *modelCluster) Get(id string) (Cluster, error) {
	c := Cluster{}
	query := "SELECT * FROM " + c.Table() + " WHERE id=?"

	err := m.dbBase.Get(&c, query, id)
	if err != nil {
		return c, err
	}

	c.Site, err = m.getSiteBrief(c.SiteID)

	return c, err
}

func (m *modelCluster) List(selector map[string]string) ([]Cluster, error) {
	if id, ok := selector["id"]; ok {

		c, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Cluster{c}, err
	}

	c := Cluster{}
	var out []Cluster

	if name, ok := selector["name"]; ok {

		query := "SELECT * FROM " + c.Table() + " WHERE name=?"

		err := m.Select(&out, query, name)
		if err != nil {
			return out, err
		}

		if len(out) == 0 {
			return out, nil
		}

		ids := make([]interface{}, 0, len(out))
	loop:
		for i := range out {
			for k := range ids {
				if ids[k] == out[i].SiteID {
					continue loop
				}
			}

			ids = append(ids, out[i].SiteID)
		}

		query = "SELECT id,name FROM " + c.Site.Table() +
			" WHERE id IN (?)"

		query, args, err := sqlx.In(query, ids)
		if err != nil {
			return out, err
		}

		var sites []SiteBrief

		err = m.Select(&sites, query, args...)
		if err != nil {
			return out, err
		}

		for i := range out {

			for k := range sites {
				if out[i].SiteID == sites[k].ID {
					out[i].Site = sites[k]
					break
				}
			}
		}

		return out, err
	}

	if site, ok := selector["site_id"]; ok {

		bs, err := m.getSiteBrief(site)
		if err != nil {
			return out, err
		}

		query := "SELECT * FROM " + c.Table() + " WHERE site_id=?"

		enabled, ok := selector[labelEnabled]
		if ok {
			query = query + " AND enabled=?"
			err = m.Select(&out, query, site, enabled)

		} else {
			err = m.Select(&out, query, site)
		}
		if err != nil {
			return out, err
		}

		for i := range out {
			out[i].Site = bs
		}

		return out, err
	}

	var err error
	query := "SELECT * FROM " + c.Table()

	enabled, ok := selector[labelEnabled]
	if ok {
		query = query + " WHERE enabled=?"
		err = m.Select(&out, query, enabled)

	} else {
		err = m.Select(&out, query)
	}
	if err != nil {
		return out, err
	}

	var sites []SiteBrief
	query = "SELECT id,name FROM " + c.Site.Table()

	err = m.Select(&sites, query)
	if err != nil {
		return out, err
	}

	for i := range out {

		for k := range sites {
			if out[i].SiteID == sites[k].ID {
				out[i].Site = sites[k]
				break
			}
		}
	}

	return out, err
}

type fakeModelCluster struct {
	sites    *sync.Map
	clusters *sync.Map
}

// Insert insert Cluster,returns Cluster.ID
func (m *fakeModelCluster) Insert(c Cluster) (string, error) {

	c.ID = newUUID(c.Name)

	m.clusters.Store(c.ID, c)

	return c.ID, nil
}

func (m *fakeModelCluster) Update(c Cluster) error {
	if c.ID == "" {
		return errors.New("id is required")
	}

	m.clusters.Store(c.ID, c)

	return nil
}

func (m *fakeModelCluster) Delete(id string) error {
	m.clusters.Delete(id)

	return nil
}
func (m *fakeModelCluster) Get(id string) (Cluster, error) {
	c, ok := m.clusters.Load(id)
	if !ok {
		return Cluster{}, NewNotFound("cluster", id)
	}

	cluster := c.(Cluster)

	v, ok := m.sites.Load(cluster.SiteID)
	if ok {
		cluster.Site.Name = v.(Site).Name
	}

	return cluster, nil
}
func (m *fakeModelCluster) List(selector map[string]string) ([]Cluster, error) {

	if id, ok := selector["id"]; ok {
		c, err := m.Get(id)

		return []Cluster{c}, err
	}

	clusters := make([]Cluster, 0, 1)

	if name, ok := selector["name"]; ok {
		m.clusters.Range(func(key, value interface{}) bool {
			v, ok := value.(Cluster)
			if ok && v.Name == name {
				clusters = append(clusters, v)
			}

			return true
		})

		return clusters, nil
	}

	if id, ok := selector["site_id"]; ok {
		m.clusters.Range(func(key, value interface{}) bool {

			v, ok := value.(Cluster)
			if ok && v.SiteID == id {

				obj, ok := m.sites.Load(v.SiteID)
				if ok {
					v.Site.Name = obj.(Site).Name
				}

				clusters = append(clusters, v)
			}

			return true
		})

		m.clusters.Range(func(key, value interface{}) bool {
			v, ok := value.(Cluster)
			if ok {

				obj, ok := m.sites.Load(v.SiteID)
				if ok {
					v.Site.Name = obj.(Site).Name
				}

				clusters = append(clusters, v)
			}

			return true
		})

		return clusters, nil
	}

	return nil, nil
}
