package model

import (
	"testing"
	"time"
)

func sampleRemoteStorage() RemoteStorage {
	return RemoteStorage{
		Enabled: randInt()%2 == 0,
		Name:    newUUID(""),
		SiteID:  newUUID(""),
		Vendor:  newUUID(""),
		Model:   newUUID(""),
		Type:    newUUID(""),
		Desc:    newUUID(""),

		Editor: Editor{
			CreatedUser:  newUUID("")[:32],
			ModifiedUser: newUUID("")[:32],
			CreatedAt:    time.Now(),
			ModifiedAt:   time.Now(),
		},
		Auth: Auth{
			Port:            randInt(),
			RemoteStorageID: newUUID(""),
			IP:              newUUID(""),
			User:            newUUID(""),
			Password:        newUUID(""),
		},
	}
}

func TestStorageModel(t *testing.T) {

	ms := db.ModelRemoteStorage()
	// rs := sampleRemoteStorage()

	// id, task, err := ms.Insert(rs)
	// if err != nil {
	// 	t.Error(err)
	// }
	// t.Log(id, task)

	storages, err := ms.List(nil)
	if err != nil {
		t.Error(err, len(storages))
	}

	for i := range storages {
		rs, err := ms.Get(storages[i].ID)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(rs)
		}
	}
}

func TestStoragePoolModel(t *testing.T) {
	mrs := db.ModelRemoteStorage()

	pools, err := mrs.ListPools(nil)
	if err != nil {
		t.Error(err, len(pools))
	}

	for i := range pools {

		pool, err := mrs.GetPool(pools[i].ID)
		if err != nil {
			t.Error(err, pool)
		}
	}

	pools, err = mrs.ListPools(map[string]string{
		"remote_storage_id": "770e0eb0-b149-45fb-8c38-2fb78ed99048",
	})
	if err != nil {
		t.Log(err, len(pools))
	}
}
