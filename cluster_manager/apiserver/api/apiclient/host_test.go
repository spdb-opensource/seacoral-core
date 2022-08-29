package apiclient

import (
	"testing"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

func TestNewHostAPI(t *testing.T) {
	if Client == nil {
		t.Skip("")
	}

	var (
		storageremodeid = "xxxx"
	)
	hostconfig := api.HostConfig{
		//	Name:          "aaa",
		Cluster:       "0a023444-9445-48a4-b959-ad45e1a989cf",
		Desc:          "xxxxxx",
		RemoteStorage: &storageremodeid,
		Enabled:       true,
		User:          "xxx",
		HostStorages: []api.HostStorage{
			{
				Paths: []string{
					"/dev/sdb",
				},
				Performance: "medium",
			},
		},
		SSHConfig: api.Auth{
			IP:       "192.168.2.1",
			Port:     22,
			User:     "xxx",
			Password: "xxxxxx",
		},
		Location: api.Location{
			Room: "room",
			Seat: "seat",
		},
		ResourceLimit: api.ResourceLimit{
			MaxUnit: 10,
			MaxUsage: api.MaxUsage{
				CPU:          80,
				Memory:       80,
				HostStorage:  80,
				NetBandwidth: 80,
			},
		},
	}
	posthostresp, err := Client.PostHost(Ctx, hostconfig)
	if err == nil {
		t.Log("PostHost success!")
		t.Log(posthostresp)
	} else {
		t.Error("PostHost err:", err)
	}

	/*	var (
			hostoptsroom    = "roomupdate"
			hostoptsseat    = "seat002"
			hostoptsdesc    = "xxxxxxxx"
			hostoptsenabled = true
			hostoptsmaxunit = 10
		)
		hostopts := api.HostOptions{
			Room:    &hostoptsroom,
			Seat:    &hostoptsseat,
			Desc:    &hostoptsdesc,
			MaxUnit: &hostoptsmaxunit,
			Enabled: &hostoptsenabled,
			User:    "xxxxx",
		}
		updatehostresp, err := Client.UpdateHost(Ctx, "ad1d6196-5840-4ddf-a754-1f5e08cde39d", hostopts)
		if err == nil {
			t.Log("UpdateHost success.")
			t.Log(updatehostresp)
		} else {
			t.Error("UpdateHost err:", err)
		}*/

	/*listhostsresp, err := Client.ListHosts(Ctx, "", "", "", "")
	if err == nil {
		t.Log("ListHosts success.")
		t.Log(listhostsresp)
	} else {
		t.Error("ListHosts err:", err)
	}*/

	/*err := Client.DeleteHost(Ctx, "ad1d6196-5840-4ddf-a")
	if err == nil {
		t.Log("DeleteHost success.")
	} else {
		t.Error("DeleteHost err:", err)
	}*/

}
