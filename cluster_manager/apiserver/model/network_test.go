package model

import (
	"testing"
)

func TestNetworkModel(t *testing.T) {
	m := db.ModelNetwork()

	nets, err := m.List(nil)
	if err != nil {
		t.Error(err)
	}

	for i := range nets {
		net, err := m.Get(nets[i].ID)
		if err != nil {
			t.Error(i, err)
		}

		t.Log(i, net)
	}

	// err = m.Delete(net.ID)
	// if err != nil {
	// 	t.Error(err)
	// }
}
