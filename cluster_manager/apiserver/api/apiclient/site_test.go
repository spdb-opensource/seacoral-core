package apiclient

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

var (
	addr   string
	Client *clientConfig
)

// go test ./... -v -args "addr=127.0.0.1:8000"
func init() {

	for k := range os.Args {

		args := strings.Split(os.Args[k], " ")

		for i := range args {

			parts := strings.SplitN(args[i], "=", 2)
			if len(parts) != 2 || parts[0] != "addr" {
				continue
			}

			_, _, err := net.SplitHostPort(parts[1])
			if err != nil {
				continue
			}

			addr = parts[1]

			cl := client.NewClient(addr, time.Second*30, nil)

			Client = &clientConfig{
				host:   addr,
				client: cl,
			}

			return
		}
	}
}

var Ctx context.Context

func TestNewSiteAPI(t *testing.T) {
	addr := "192.168.137.1:8080"
	cl := client.NewClient(addr, time.Second*30, nil)

	Client = &clientConfig{
		host:   addr,
		client: cl,
	}

	if Client == nil {
		t.Skip("")
	}

	siteconfig := api.SiteConfig{
		Name:   "test-site001",
		Type:   "kubernetes",
		Domain: "cluster_engine1.service.consul",
		Port:   6443,
		Desc:   "铭光测试站点-site001",
		Region: "SH",
		User:   "user001-用户",
	}

	addsiteresp, err := Client.AddSite(Ctx, siteconfig)
	if err == nil {
		t.Log("AddSite success!")
		t.Log(addsiteresp)
	} else {
		t.Error("AddSite err:", err)
	}

	var (
		siteoptsname   = "test-site002-铭光"
		siteoptsdomain = "cluster_engine1.service.consul"
		siteoptsport   = 6448
		siteoptsdesc   = "铭光测试站点-site002"
		siteoptsregion = "SH"
	)
	siteopts := api.SiteOptions{
		Name:   &siteoptsname,
		Domain: &siteoptsdomain,
		Port:   &siteoptsport,
		Desc:   &siteoptsdesc,
		Region: &siteoptsregion,
		User:   "user002-用户",
	}
	err = Client.UpdateSite(Ctx, "7a8307e1-30d3-4aa5-b920-ae5796ac36e5", siteopts)
	if err == nil {
		t.Log("UpdateSite success!")
	} else {
		t.Error("UpdateSite err:", err)
	}

	listsiteresp, err := Client.ListSites(Ctx, "8c55b249-d3a7-4690-a085-3c8cbcc31958", "")
	if err == nil {
		t.Log("ListSites success!")
		t.Log(listsiteresp)
	} else {
		t.Error("ListSites err:", err)
	}

	/*err := Client.DeleteSite(Ctx, "41730f30-e6c1-45be-82e5-d49f2542cbe2")
	if err == nil {
		t.Log("DeleteSite success!")
	} else {
		t.Error("DeleteSite err:", err)
	}*/
}
