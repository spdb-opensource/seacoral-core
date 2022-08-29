package apiclient

import (
	"testing"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/client"
)

func TestNewAppAPI(t *testing.T) {
	addr := "192.168.137.1:8080"
	cl := client.NewClient(addr, time.Second*30, nil)

	Client = &clientConfig{
		host:   addr,
		client: cl,
	}

	if Client == nil {
		t.Skip("")
	}

	//AddApp
	optionsmap := make(map[string]string)
	optionsmap["character_set_server"] = "utf8mb4"
	var configbandwidth int32
	configbandwidth = 100
	var configstorage = api.StorageRequirement{
		Type:        "host",
		Performance: "high",
		Volumes: []api.VolumeRequirement{
			{
				Capacity: 10240,
				Type:     "data",
			},
			{
				Capacity: 10240,
				Type:     "log",
			},
		},
	}

	appdatabase := api.GroupSpec{
		Image: api.ImageVersion{
			ID: "app001",
		},
		Services: api.ServiceSpec{
			Num: 1,
			Arch: api.Arch{
				Replicas: 1,
				Mode:     "single",
			},
			Ports: []api.ContainerPort{
				{
					Name: "server",
					Port: 3306,
				},
			},
			Options: optionsmap,
			Conditions: api.Conditions{
				Cluster: api.Condition{
					Candidates: []string{
						"string",
					},
					HighAvailability: true,
				},
				Host: api.Condition{
					Candidates: []string{
						"string",
					},
					HighAvailability: true,
				},
				Network: api.Condition{
					Candidates: []string{
						"string",
					},
					HighAvailability: true,
				},
				RemoteStorage: api.Condition{
					Candidates: []string{
						"string",
					},
					HighAvailability: true,
				},
			},
			Backup: api.Backup{
				Storage: api.BackupStorage{
					Type:        "remote",
					Performance: "medium",
					Capacity:    1000,
				},
			},
			Units: api.UnitSpec{
				Resources: api.Resources{
					Requests: api.ResourceRequirements{
						CPU:       500,
						Memory:    256,
						Bandwidth: &configbandwidth,
						Storage:   &configstorage,
					},
				},
			},
		},
	}
	appconfig := api.AppConfig{
		Name: "app001",
		Desc: "xxxxxxxx",
		User: "xxxxx",
		Spec: api.AppSpec{
			Database: &appdatabase,
		},
	}
	addappresp, err := Client.AddApp(Ctx, appconfig)
	if err == nil {
		t.Log("AddApp success.")
		t.Log(addappresp)
	} else {
		t.Error("AddApp err:", err)
	}

	//ListApps
	listappsresp, err := Client.ListApps(Ctx, "app001", "001")
	if err == nil {
		t.Log("ListApps success.")
		t.Log(listappsresp)
	} else {
		t.Error("ListApps err:", err)
	}

	/*type Image struct {
		api.ImageVersion
	}
	var imageoptsimageversion Image
	imageoptsimageversion=Image{
		ImageVersion:api.ImageVersion{
			BackupType:"upredis",
			Major:2,
			Minor:0,
			Patch:0,
		},
	}
	type Database struct {
		*Image
	}

	type Spec struct {
		*Database
	}

	imageopts:= api.AppImageOptions{
		Spec: &Spec{
			Database:&Image{
				&imageoptsimageversion
				}
			}
		}
	}

	Client.UpdateAppImage(Ctx, "app001")*/

	//更改服务资源需求=资源请求变更
	/*vol := []api.VolumeRequirement{
		{Capacity: 100,
			BackupType: "data",
		},
	}*/
	/*var cpu float32
	cpu = 0.5
	var menory int64
	menory = 1000
	var bandwidth int64
	bandwidth = 100*/

	var uaropts api.AppResourcesOptions
	/*uaropts.Spec.Database.Resources.Requests.Bandwidth = &bandwidth
		uaropts.Spec.Database.Resources.Requests.CPU = &cpu
		uaropts.Spec.Database.Resources.Requests.Memory = &menory
	    uaropts.Spec.Database.Resources.Requests.Storage.Volumes = vol*/
	uarresp, err := Client.UpdateAppResources(Ctx, "app001", uaropts)
	if err == nil {
		t.Log("UpdateAppResources success.")
		t.Log(uarresp)
	} else {
		t.Error("UpdateAppResources err:", err)
	}

	/*//更改服务状态,启动或停止=运行状态变更
	stateopts := api.AppStateOptions{
		State: "passing",
	}
	updateappstateresp, err := Client.UpdateAppState(Ctx, "app001", stateopts)
	if err == nil {
		t.Log("UpdateAppState success.")
		t.Log(updateappstateresp)
	} else {
		t.Error("UpdateAppState err:", err)
	}*/

}
