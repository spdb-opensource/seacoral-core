package model

import (
	"testing"
)

func sampleCluster(site string) Cluster {
	if site == "" {
		site = newUUID(`db:"site_id"`)
	}

	return Cluster{
		Enabled: randInt()%2 == 0,
		ID:      newUUID("id"),
		Name:    newUUID("name"),
		SiteID:  site,
		Zone:    newUUID("zone"),
		HaTag:   newUUID("ha_tag"),
		Desc:    newUUID("description"),
		Images:  NewSliceString([]string{newUUID("image_type"), newUUID("image_type")}),
		Editor:  sampleCreateEditor(),
	}
}

func TestModelCluster(t *testing.T) {
	m := 3
	sites := make([]Site, 0, m)
	ms := db.ModelSite()

	for i := 0; i < m; i++ {
		s := sampleSite()

		id, err := ms.Insert(s)
		if err != nil {
			t.Error(i, err)
		} else {
			s.ID = id
			sites = append(sites, s)
		}
	}

	m = len(sites)

	n := 10 * m
	clusters := make([]Cluster, 0, n)
	mc := db.ModelCluster()

	for i := 0; i < n; i++ {
		c := sampleCluster(sites[i%m].ID)

		id, err := mc.Insert(c)
		if err != nil {
			t.Error(i, err)
		} else {
			c.ID = id
			clusters = append(clusters, c)
		}
	}

	n = len(clusters)

	for i := range clusters {

		clusters[i].Enabled = randInt()%2 == 0
		clusters[i].Name = newUUID("update")
		clusters[i].Zone = newUUID("update")
		clusters[i].HaTag = newUUID("update")
		clusters[i].Desc = newUUID("update")
		clusters[i].Images = SliceString(newUUID("update"))
		clusters[i].Editor.setModified()

		err := mc.Update(clusters[i])
		if err != nil {
			t.Error(i, err)
		}
	}

	for i := range clusters {
		c, err := mc.Get(clusters[i].ID)
		if err != nil {
			t.Error(i, err)
		}

		// t.Log(c)

		if testDB &&
			(c.SiteID != sites[i%m].ID ||
				c.Site.Name != sites[i%m].Name) {

			t.Errorf("\nwant: %s %s\ngot : %s %s",
				sites[i%m].ID, sites[i%m].Name,
				c.SiteID, c.Site.Name)
		}

		out, err := mc.List(map[string]string{
			"name": clusters[i].Name,
		})
		if err != nil {
			t.Error(i, err)
		}

		if testDB && len(out) != 1 &&
			out[0].ID != clusters[i].ID {

			t.Errorf("Unexpected,got %v", out)
		}
	}

	for i := range sites {

		out, err := mc.List(map[string]string{
			"site_id": sites[i].ID,
		})

		if err != nil {
			t.Error(i, err)
		}

		if testDB && len(out) != n/m {
			t.Errorf("want:%d,got %d", n/m, len(out))
		}
	}

	list, err := mc.List(map[string]string{})
	if err != nil {
		t.Error(err)
	}

	if testDB && len(list) != m {
		t.Logf("want:%d,got %d", m, len(list))
	}

	list1, err := mc.List(nil)
	if err != nil {
		t.Error(err)
	}

	if testDB && len(list1) != m {
		t.Logf("want:%d,got %d", m, len(list1))
	}

	for i := range clusters {
		if i%2 == 0 {
			err := mc.Delete(clusters[i].ID)
			if err != nil {
				t.Error(i, err)
			}
		}
	}
}

func TestClusterList(t *testing.T) {
	mc := db.ModelCluster()

	list, err := mc.List(map[string]string{})
	if err != nil {
		t.Error(err)
	}

	if testDB {
		t.Logf("got %d", len(list))
	}

	out, err := mc.List(map[string]string{
		"name": "test-集群0000",
	})
	if err != nil {
		t.Error(err)
	}

	if testDB {
		t.Logf("got %d", len(out))
	}

	out1, err := mc.List(map[string]string{
		"site_id": "ef30e7f6-7ae3-4f15-a9eb-57d30033e57c",
	})

	if err != nil {
		t.Error(err)
	}

	if testDB {
		t.Logf("got %d", len(out1))
	}
}
