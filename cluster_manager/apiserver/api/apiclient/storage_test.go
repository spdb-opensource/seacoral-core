package apiclient

import (
	"testing"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

func TestNewRemoteAPI(t *testing.T) {
	if Client == nil {
		t.Skip("")
	}

	remotestorageconfig := api.RemoteStorageConfig{
		Name:    "storage001",
		Site:    "xxxxx",
		Desc:    "xxxxxxxx",
		Type:    "FC",
		Model:   "OceanStorV3",
		Enabled: true,
		User:    "xxx",
		Vendor:  "HUAWEI",
		Auth: api.Auth{
			IP:       "192.168.10.10",
			Port:     2222,
			User:     "xxxx",
			Password: "xxxx",
		},
	}
	_, err := Client.PostRemoteStorage(Ctx, remotestorageconfig)
	if err != nil {
		t.Error("PostRemoteStorage err:", err)
	}

	var (
		rsoname   = "storage001"
		rsodesc   = "xxxxxxx"
		rsoenable = true
	)

	remotestorageopts := api.RemoteStorageOptions{
		Name:    &rsoname,
		Desc:    &rsodesc,
		User:    "xxx",
		Enabled: &rsoenable,
	}
	_, err = Client.UpdateRemoteStorage(Ctx, "storage001", remotestorageopts)
	if err == nil {
		t.Log("UpdateRemoteStorage success!")
	} else {
		t.Error("UpdateRemoteStorage err:", err)
	}

	listremotestorageresp, err := Client.ListRemoteStorages(Ctx, "storage001", "storage001", "铭光测试站点-site002")
	if err == nil {
		t.Log("ListRemoteStorages success!")
		t.Log(listremotestorageresp)
	} else {
		t.Error("ListRemoteStorages err:", err)
	}

	err = Client.DeleteRemoteStorage(Ctx, "site002-铭光")
	if err == nil {
		t.Log("DeleteRemoteStorage success!")
	} else {
		t.Error("DeleteRemoteStorage err:", err)
	}

	//remotestoragepool
	remotestoragepoolconfig := api.RemoteStoragePoolConfig{
		Name:    "storage_pool001",
		Native:  "xxxx",
		Enabled: true,
		Desc:    "xxxxxxx",
		User:    "xxx",
	}
	_, err = Client.PostRemoteStoragePool(Ctx, "storage001", remotestoragepoolconfig)
	if err != nil {
		t.Error("PostRemoteStoragePool err:", err)
	}

	var (
		rspoptsname    = "storage_pool001"
		rspoptsenabled = true
		rspoptsdesc    = "xxxxxxx"
	)
	remotestoragepoolopts := api.RemoteStoragePoolOptions{
		Name:    &rspoptsname,
		Enabled: &rspoptsenabled,
		Desc:    &rspoptsdesc,
		User:    "xxx",
	}
	_, err = Client.UpdateRemoteStoragePool(Ctx, "storage001", "storage_pool001", remotestoragepoolopts)
	if err == nil {
		t.Log("UpdateRemoteStoragePool success!")
	} else {
		t.Error("UpdateRemoteStoragePool err:", err)
	}

	listremotestoragepoolresp, err := Client.ListRemoteStoragePools(Ctx, "000", "storagepool001", "storage001")
	if err == nil {
		t.Log("ListRemoteStoragePools success!")
		t.Log(listremotestoragepoolresp)
	} else {
		t.Error("ListRemoteStoragePools err:", err)
	}

	err = Client.DeleteRemoteStoragePool(Ctx, "storage001", "storage001pool")
	if err == nil {
		t.Log("DeleteRemoteStoragePool success!")
	} else {
		t.Error("DeleteRemoteStoragePool err:", err)
	}
}
