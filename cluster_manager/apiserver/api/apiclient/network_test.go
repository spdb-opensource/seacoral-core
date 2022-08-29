package apiclient

import (
	"testing"
)

func TestNewNetworkAPI(t *testing.T) {
	if Client == nil {
		t.Skip("")
	}

	/*networkconfig := api.NetworkConfig{
		Name:    "network001",
		Cluster: "79c19ee2-8161-4e23-a30e-2e16fb4106e2",
		Desc:    "network001-网段",
		Topology: []string{
			"topo001-上海",
		},
		IPSummary: api.Route{
			Prefix:  24,
			VLan:    100,
			Start:   "10.0.0.101",
			End:     "10.0.0.150",
			Gateway: "10.0.0.254",
		},
		Enabled: true,
		User:    addr,
	}
	postnetworkresp, err := Client.PostNetwork(Ctx, networkconfig)
	if err == nil {
		t.Log("PostNetwork success!")
		t.Log(postnetworkresp)
	} else {
		t.Error("PostNetwork err:", err)
	}*/

	/*var (
		networkoptsname    = "network002-网段"
		networkoptscluster = "xxxx"
		networkoptsdesc    = "network001-网段"
		networkoptsenabled = false
	)
	networkopts := api.NetworkOptions{
		Name:    &networkoptsname,
		Cluster: &networkoptscluster,
		Desc:    &networkoptsdesc,
		Topology: []string{
			"topo002-北京",
		},
		Enabled: &networkoptsenabled,
		User:    "user002-用户",
	}

	_, err = Client.UpdateNetwork(Ctx, "network002-网段", networkopts)
	if err == nil {
		t.Log("UpdateNetwork success!")
	} else {
		t.Error("UpdateNetwork err:", err)
	}*/

	/*listnetworkresp, err := Client.ListNetworks(Ctx, "", "", "", "", "")
	if err == nil {
		t.Log("ListNetworks success!")
		t.Log(listnetworkresp)
	} else {
		t.Error("ListNetworks err:", err)
	}*/

	err := Client.DeleteNetwork(Ctx, "b769fb38-e359-4dfe-9f9b-7788851ddf2a")
	if err == nil {
		t.Log("DeleteNetwork success!")
	} else {
		t.Error("DeleteNetwork err:", err)
	}
}
