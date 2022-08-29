package model

import (
	"testing"
	"time"
)

func sampleCreateEditor() Editor {
	return Editor{
		CreatedUser: newUUID("created_user"),
		CreatedAt:   time.Now(),
	}
}

func (e *Editor) setModified() {
	e.ModifiedUser = newUUID("modified_user")
	e.ModifiedAt = time.Now().Add(time.Duration(randInt()))
}

func sampleSite() Site {
	return Site{
		Port: randInt(),
		//	ID:           newUUID(""),
		Name:   newUUID("name"),
		Type:   newUUID("type"),
		Desc:   newUUID("desc"),
		Domain: newUUID("domain"),
		Region: newUUID("region"),
		Config: newUUID("config"),
		Editor: sampleCreateEditor(),
	}
}

func TestModelSite(t *testing.T) {
	n := 10
	sites := make([]Site, 0, n)
	ms := db.ModelSite()

	for i := 0; i < n; i++ {
		s := sampleSite()

		id, err := ms.Insert(s)
		if err != nil {
			t.Error(i, err)
		} else {
			s.ID = id
			sites = append(sites, s)
		}
	}

	n = len(sites)

	for i := range sites {

		sites[i].Name = newUUID("update")
		sites[i].Domain = newUUID("update")
		sites[i].Desc = newUUID("update")
		sites[i].Config = newUUID("update")
		sites[i].Region = newUUID("update")
		sites[i].Port = randInt()
		sites[i].Editor.setModified()

		err := ms.Update(sites[i])
		if err != nil {
			t.Error(i, err)
		}
	}

	for i := range sites {
		s, err := ms.Get(sites[i].ID)
		if err != nil {
			t.Error(i, err)
		}

		if testDB && sites[i].Name != s.Name {
			t.Errorf("%d: want %s got %s", i, sites[i].Name, s.Name)
		}
	}

	for i := range sites {
		out, err := ms.List(map[string]string{"id": sites[i].ID})
		if err != nil {
			t.Error(i, err)
		}

		if testDB &&
			(len(out) != 1 || sites[i].ID != out[0].ID) {
			t.Errorf("%d: want %s got %d", i, sites[i].Name, len(out))
		}

		out, err = ms.List(map[string]string{"name": sites[i].Name})
		if err != nil {
			t.Error(i, err)
		}
	}

	out, err := ms.List(map[string]string{})
	if err != nil {
		t.Error(err)
	}

	if testDB && len(out) != n {
		t.Logf("want %d,got %d", n, len(out))
	}

	out1, err := ms.List(nil)
	if err != nil {
		t.Error(err)
	}

	if testDB && len(out1) != n {
		t.Logf("want %d,got %d", n, len(out1))
	}

	for i := range sites {
		if i%2 == 0 {
			continue
		}

		err := ms.Delete(sites[i].ID)
		if err != nil {
			t.Error(i, err)
		}
	}
}
