package model

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
)

type Network struct {
	ID        string      `db:"id"`
	Name      string      `db:"name"`
	ClusterID string      `db:"cluster_id"`
	Desc      string      `db:"description"`
	Topology  SliceString `db:"topology"`
	Enabled   bool        `db:"enabled"`

	Editor
	Site    SiteBrief    `db:"-"`
	Cluster ClusterBrief `db:"-"`
}

func (Network) Table() string {
	return "tbl_network"
}

func (net Network) ObjectName() string {
	return fmt.Sprintf("%s", net.ID)
}

func NewNetworkModel() ModelNetwork {
	return &modelNetwork{}
}

type modelNetwork struct {
	*dbBase
}

// Insert insert Network,returns Network.ID
func (m *modelNetwork) Insert(net Network) (string, error) {
	if net.ID == "" {
		net.ID = newUUID(net.Name)
	}

	query := "INSERT INTO " + net.Table() +
		" (id,name,cluster_id,topology,enabled,description,created_user,created_timestamp,modified_user,modified_timestamp) " +
		"VALUES (:id,:name,:cluster_id,:topology,:enabled,:description,:created_user,:created_timestamp,:modified_user,:modified_timestamp)"

	_, err := m.NamedExec(query, net)

	return net.ID, err
}

func (m *modelNetwork) Update(net Network) error {
	query := "UPDATE " + net.Table() +
		" SET cluster_id=:cluster_id,topology=:topology,enabled=:enabled,description=:description,modified_user=:modified_user,modified_timestamp=:modified_timestamp " +
		"WHERE id=:id"

	_, err := m.NamedExec(query, net)

	return err
}

func (m *modelNetwork) Delete(id string) error {
	query := "DELETE FROM " + Network{}.Table() + " WHERE id=?"

	_, err := m.Exec(query, id)
	if IsNotExist(err) {
		return nil
	}

	return err
}

func (m *modelNetwork) Get(id string) (Network, error) {
	net := Network{}
	query := "SELECT * FROM " + net.Table() + " WHERE id=?"

	err := m.dbBase.Get(&net, query, id)
	if err != nil {
		return net, err
	}

	net.Cluster, err = m.getClusterBrief(net.ClusterID)
	if err != nil {
		return net, err
	}

	net.Site, err = m.getSiteBrief(net.Cluster.SiteID)

	return net, err
}

func (m *modelNetwork) List(selector map[string]string) ([]Network, error) {
	if id, ok := selector["id"]; ok {

		net, err := m.Get(id)
		if IsNotExist(err) {
			return nil, nil
		}

		return []Network{net}, err
	}

	if name, ok := selector["name"]; ok {

		nets := []Network{}
		query := "SELECT * FROM " + Network{}.Table() + " WHERE name=?"

		err := m.Select(&nets, query, name)
		if err != nil {
			return nil, err
		}

		for i := range nets {
			nets[i].Cluster, _ = m.getClusterBrief(nets[i].ClusterID)
			nets[i].Site, _ = m.getSiteBrief(nets[i].Cluster.SiteID)
		}

		return nets, nil
	}

	if cluster, ok := selector["cluster_id"]; ok {
		cb, err := m.getClusterBrief(cluster)
		if err != nil {
			return nil, err
		}

		nets := []Network{}
		query := "SELECT * FROM " + Network{}.Table() + " WHERE cluster_id=?"

		if enabled, ok := selector[labelEnabled]; ok {
			query = query + " AND enabled=?"
			err = m.Select(&nets, query, cluster, enabled)
		} else {
			err = m.Select(&nets, query, cluster)
		}
		if err != nil {
			return nil, err
		}

		bs, _ := m.getSiteBrief(cb.SiteID)

		for i := range nets {
			nets[i].Cluster = cb
			nets[i].Site = bs
		}

		return nets, nil
	}

	if site, ok := selector["site_id"]; ok {
		bs, err := m.getSiteBrief(site)
		if err != nil {
			return nil, err
		}

		clusters := []ClusterBrief{}
		query := "SELECT id,name,site_id FROM " + Cluster{}.Table() +
			" WHERE site_id=?"

		err = m.Select(&clusters, query, site)
		if err != nil {
			return nil, err
		}

		if len(clusters) == 0 {
			return nil, nil
		}

		nets := []Network{}
		ids := make([]interface{}, len(clusters))
		query = "SELECT * FROM " + Network{}.Table() +
			" WHERE cluster_id IN (?)"

		for i := range clusters {
			ids[i] = clusters[i].ID
		}

		query, args, err := sqlx.In(query, ids)
		if err != nil {
			return nil, err
		}

		err = m.Select(&nets, query, args...)
		if err != nil {
			return nil, err
		}

		if enabled, ok := selector[labelEnabled]; ok {
			v := enabled == "1"
			list := make([]Network, 0, len(nets))

			for i := range nets {
				if nets[i].Enabled == v {
					list = append(list, nets[i])
				}
			}

			nets = list
		}

		for i := range nets {
			nets[i].Site = bs

			for k := range clusters {
				if nets[i].ClusterID == clusters[k].ID {
					nets[i].Cluster = clusters[k]
					break
				}
			}
		}

		return nets, nil
	}

	var err error
	nets := []Network{}
	query := "SELECT * FROM " + Network{}.Table()

	if enabled, ok := selector[labelEnabled]; ok {
		query = query + " WHERE enabled=?"
		err = m.Select(&nets, query, enabled)
	} else {
		err = m.Select(&nets, query)
	}
	if err != nil {
		return nil, err
	}

	sites := []SiteBrief{}
	query = "SELECT id,name FROM " + Site{}.Table()
	err = m.Select(&sites, query)
	if err != nil {
		return nil, err
	}

	clusters := []ClusterBrief{}
	query = "SELECT id,name,site_id FROM " + Cluster{}.Table()

	err = m.Select(&clusters, query)
	if err != nil {
		return nil, err
	}

	for i := range nets {
		for k := range clusters {
			if nets[i].ClusterID == clusters[k].ID {
				nets[i].Cluster = clusters[k]

				break
			}
		}

		for n := range sites {
			if nets[i].Cluster.SiteID == sites[n].ID {
				nets[i].Site = sites[n]
				break
			}
		}
	}

	return nets, nil
}

type fakeModelNetwork struct {
	networks *sync.Map
	clusters *sync.Map
}

// Insert insert Network,returns Network.ID
func (m *fakeModelNetwork) Insert(nw Network) (string, error) {
	nw.ID = newUUID(nw.Name)

	m.networks.Store(nw.ID, nw)

	return nw.ID, nil
}

func (m *fakeModelNetwork) Update(nw Network) error {
	if nw.ID == "" {
		return errors.New("id is required")
	}

	m.networks.Store(nw.ID, nw)

	return nil
}

func (m *fakeModelNetwork) Delete(id string) error {
	m.networks.Delete(id)

	return nil
}

func (m *fakeModelNetwork) Get(id string) (Network, error) {
	v, ok := m.networks.Load(id)
	if !ok {
		return Network{}, NewNotFound("network", id)
	}

	nw := v.(Network)

	v, ok = m.clusters.Load(nw.ClusterID)
	if ok {
		c := v.(Cluster)

		nw.Cluster.Name = c.Name
		nw.Cluster.SiteID = c.SiteID
	}

	return nw, nil
}

func (m *fakeModelNetwork) List(selector map[string]string) ([]Network, error) {

	if id, ok := selector["id"]; ok {
		nw, err := m.Get(id)

		return []Network{nw}, err
	}

	networks := make([]Network, 0, 2)

	if name, ok := selector["name"]; ok {

		m.networks.Range(func(key, value interface{}) bool {

			v, ok := value.(Network)
			if ok && v.Name == name {
				networks = append(networks, v)
			}

			return true
		})

		for i := range networks {

			v, ok := m.clusters.Load(networks[i].ClusterID)
			if ok {
				c := v.(Cluster)
				networks[i].Cluster.Name = c.Name
				networks[i].Cluster.SiteID = c.SiteID
			}
		}

		return networks, nil
	}

	clusters := make([]Cluster, 0, 2)
	clusterID := selector["cluster_id"]
	siteID := selector["site_id"]

	m.clusters.Range(func(key, value interface{}) bool {

		c, ok := value.(Cluster)

		if ok &&
			(len(selector) == 0 ||
				(clusterID != "" && c.ID == clusterID) ||
				(siteID != "" && c.SiteID == siteID)) {

			clusters = append(clusters, c)
		}

		return true
	})

	m.networks.Range(func(key, value interface{}) bool {

		v, ok := value.(Network)
		if !ok {
			return true
		}

		for i := range clusters {
			if v.ClusterID == clusters[i].ID {

				v.Cluster.Name = clusters[i].Name
				v.Cluster.SiteID = clusters[i].SiteID

				networks = append(networks, v)

				break
			}
		}

		return true
	})

	return networks, nil
}
