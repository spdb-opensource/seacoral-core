package apiclient

import (
	"testing"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
)

func TestNewClusterAPI(t *testing.T) {
	if Client == nil {
		t.Skip("")
	}

	clusterconfig := api.ClusterConfig{
		Name: "test-集群001",
		Site: "8c55b249-d3a7-4690-a085-3c8cbcc31958",
		Zone: "testzoon001-测试一区",
		Images: []string{
			"mysql",
		},
		HATag:   "slice-模块001",
		Desc:    "test-集群001",
		User:    "user001-用户",
		Enabled: true,
	}
	postclusterresp, err := Client.PostCluster(Ctx, clusterconfig)
	if err == nil {
		t.Log("PostCluster success!")
		t.Log(postclusterresp)
	} else {
		t.Error("PostCluster err:", err)
	}

	/*var (
		clusteroptsname    = "test-集群测试"
		clusteroptszone    = "testzoon002-测试二区"
		clusteroptshatag   = "slice-模块002"
		clusteroptsenabled = true
		clusteroptsdesc    = "test-集群002"
	)

	clusteropts := api.ClusterOptions{
		Name: &clusteroptsname,
		Zone: &clusteroptszone,
		Images: []string{
			"mysql",
		},
		HaTag:   &clusteroptshatag,
		Enabled: &clusteroptsenabled,
		Desc:    &clusteroptsdesc,
		User:    "user002-用户",
	}
	updateclusterresp, err := Client.UpdateCluster(Ctx, "6a5c4de3-14c6-4020-93b7-4d429703e7fd", clusteropts)
	if err == nil {
		t.Log("UpdateCluster success!")
		t.Log(updateclusterresp)
	} else {
		t.Error("UpdateCluster err:", err)
	}*/

	/*listclusterresp, err := Client.ListClusters(Ctx, "", "test-集群001", "0a3b6377-9a84-4d1f-be9c-b2a8eaacf89b")
	if err == nil {
		t.Log("ListClusters success!")
		t.Log(listclusterresp)
	} else {
		t.Error("ListClusters err:", err)
	}*/

	/*err := Client.DeleteCluster(Ctx, "4ab2ef43-d527-4319-b5db-d47e375c676a")
	if err == nil {
		t.Log("DeleteCluster success!")
	} else {
		t.Error("DeleteCluster err:", err)
	}*/
}
