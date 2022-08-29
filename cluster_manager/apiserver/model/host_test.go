package model

import (
	"testing"
)

func TestHostModel(t *testing.T) {
	mh := db.ModelHost()

	list, err := mh.List(nil)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	for i := range list {
		//	t.Log("origin",i,list[i])

		h, err := mh.Get(list[i].ID)
		if err != nil {
			t.Errorf("%s %+v", list[i].ID, err)
		} else {
			t.Log(i, h)
		}
	}

	list, err = mh.List(map[string]string{
		"site_id": "8c55b249-d3a7-4690-a085-3c8cbcc31958",
	})
	if err != nil {
		t.Log(err)
	}
}
