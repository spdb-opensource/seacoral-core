package model

import (
	"testing"
)

func TestImageModel(t *testing.T) {

	mi := db.ModelImage()

	list, err := mi.List(nil)
	if err != nil {
		t.Error(err)
	}

	for i := range list {

		im, err := mi.Get(list[i].ID)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(im)
		}
	}

	list, err = mi.List(map[string]string{
		"type":  "mysql",
		"major": "0",
	})
	if err != nil {
		t.Error(err)
	}

	list, err = mi.List(map[string]string{
		labelEnabled: "1",
	})
	if err != nil {
		t.Error(err)
	}

	for i := range list {

		im, err := mi.Get(list[i].ID)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(im)
		}
	}

}
