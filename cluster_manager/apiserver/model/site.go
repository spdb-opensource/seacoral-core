package model

import (
	"errors"
	"sync"
)

type Site struct {
	Port          int    `db:"port"`
	ID            string `db:"id"`
	Name          string `db:"name"`
	Type          string `db:"type"`
	Desc          string `db:"description"`
	Domain        string `db:"domain"`
	Region        string `db:"region"`
	Config        string `db:"config"`
	ImageRegistry string `db:"image_registry"`
	ProjectName   string `db:"project_name"`
	NetworkMode   string `db:"network_mode"`

	Editor
}

func (Site) Table() string {
	return "tbl_site"
}

type SiteBrief struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

func (SiteBrief) Table() string {
	return Site{}.Table()
}

func NewSiteModel() ModelSite {
	return &modelSite{}
}

type modelSite struct {
	*dbBase
}

// Insert returns Site ID and error
func (m *modelSite) Insert(s Site) (string, error) {
	if s.ID == "" {
		s.ID = newUUID(s.Name)
	}

	query := "INSERT INTO " + s.Table() +
		" (id,name,type,domain,port,description,region,config,image_registry,project_name,network_mode,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:name,:type,:domain,:port,:description,:region,:config,:image_registry,:project_name,:network_mode,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	_, err := m.NamedExec(query, s)
	if err != nil {
		return "", err
	}

	return s.ID, nil
}

func (m *modelSite) Update(s Site) error {
	query := "UPDATE " + s.Table() +
		" SET name=:name,domain=:domain,port=:port,description=:description," +
		"region=:region,config=:config,image_registry=:image_registry, project_name=:project_name, network_mode=:network_mode, " +
		"modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, s)

	return err
}

func (m *modelSite) Delete(id string) error {
	query := "DELETE FROM " + Site{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (db *dbBase) getSiteBrief(id string) (SiteBrief, error) {
	s := SiteBrief{}
	query := "SELECT id,name FROM " + s.Table() + " WHERE id=?"

	if id == "" {
		return s, nil
	}

	err := db.Get(&s, query, id)

	return s, err
}

func (m *modelSite) Get(id string) (Site, error) {
	s := Site{}
	query := "SELECT * FROM " + s.Table() + " WHERE id=?"

	err := m.dbBase.Get(&s, query, id)

	return s, err
}

func (m *modelSite) List(selector map[string]string) ([]Site, error) {
	if id, ok := selector["id"]; ok {

		s, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Site{s}, err
	}

	if name, ok := selector["name"]; ok {

		out := []Site{}
		query := "SELECT * FROM " + Site{}.Table() + " WHERE name=?"

		err := m.Select(&out, query, name)

		return out, err
	}

	var out []Site

	query := "SELECT * FROM " + Site{}.Table()

	err := m.Select(&out, query)

	return out, err
}

type fakeModelSite struct {
	sites *sync.Map
}

// Insert returns Site ID and error
func (m *fakeModelSite) Insert(s Site) (string, error) {
	s.ID = newUUID(s.Name)

	m.sites.Store(s.ID, s)

	return s.ID, nil
}

func (m *fakeModelSite) Update(s Site) error {
	if s.ID == "" {
		return errors.New("id is required")
	}

	m.sites.Store(s.ID, s)

	return nil
}

func (m *fakeModelSite) Delete(id string) error {
	m.sites.Delete(id)

	return nil
}

func (m *fakeModelSite) Get(id string) (Site, error) {
	v, ok := m.sites.Load(id)
	if !ok {
		return Site{}, NewNotFound("site", id)
	}

	return v.(Site), nil
}

func (m *fakeModelSite) List(selector map[string]string) ([]Site, error) {
	if id, ok := selector["id"]; ok {
		s, err := m.Get(id)

		return []Site{s}, err
	}

	sites := make([]Site, 0, 1)

	if name, ok := selector["name"]; ok {

		m.sites.Range(func(key, value interface{}) bool {

			v, ok := value.(Site)
			if ok && v.Name == name {
				sites = append(sites, v)
			}

			return true
		})

		return sites, nil
	}

	m.sites.Range(func(key, value interface{}) bool {
		v, ok := value.(Site)
		if ok {
			sites = append(sites, v)
		}

		return true
	})

	return sites, nil
}
