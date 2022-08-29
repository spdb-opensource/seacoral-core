package model

import (
	"sync"
)

var fake = NewFakeModels()

type fakeModels struct {
	sites *sync.Map

	clusters *sync.Map

	networks *sync.Map

	images *sync.Map

	hosts        *sync.Map
	hostStorages *sync.Map

	storages *sync.Map // remote storages
	auths    *sync.Map
	pools    *sync.Map

	apps  *sync.Map
	units *sync.Map

	tasks *sync.Map
}

func NewFakeModels() *fakeModels {
	return &fakeModels{
		sites:    new(sync.Map),
		clusters: new(sync.Map),
		networks: new(sync.Map),
		hosts:    new(sync.Map),
		images:   new(sync.Map),
		storages: new(sync.Map),
		apps:     new(sync.Map),
		units:    new(sync.Map),
		tasks:    new(sync.Map),
	}
}

func (f *fakeModels) ModelCluster() ModelCluster {
	return &fakeModelCluster{
		sites:    f.sites,
		clusters: f.clusters,
	}
}

func (f *fakeModels) ModelSite() ModelSite {
	return &fakeModelSite{
		sites: f.sites,
	}
}

func (f *fakeModels) ModelNetwork() ModelNetwork {
	return &fakeModelNetwork{
		networks: f.networks,
		clusters: f.clusters,
	}
}

func (f *fakeModels) ModelHost() ModelHost {
	return &fakeModelHost{
		clusters: f.clusters,
		hosts:    f.hosts,
		hss:      f.hostStorages,
		rss:      f.storages,
		tasks:    f.ModelTask(),
	}
}

func (f *fakeModels) ModelImage() ModelImage {
	return &fakeModelImage{
		images: f.images,
		tasks:  f.ModelTask(),
	}
}

func (f *fakeModels) ModelTask() ModelTask {
	return &fakeModelTask{
		tasks: f.tasks,
	}
}

func (f *fakeModels) ModelRemoteStorage() ModelRemoteStorage {
	return &fakeModelRemoeteStorage{
		rs:    f.storages,
		sites: f.sites,
		auths: f.auths,
		pools: f.pools,
		tasks: f.ModelTask(),
	}
}

// func (f *fakeModels) ModelRemoteStoragePool() ModelRemoteStoragePool {
// 	return &fakeModelRemoeteStoragePool{
// 		rs:    f.storages,
// 		pools: f.pools,
// 		tasks: f.ModelTask(),
// 	}
// }

func (f *fakeModels) ModelApp() ModelApp {
	return &fakeModelApp{
		apps:  f.apps,
		units: f.units,
		hosts: f.ModelHost(),
		tasks: f.ModelTask(),
	}
}

func (fakeModels) ModelBackupStrategy() ModelBackupStrategy {
	return &fakeModelBackupStrategy{}
}

func (fakeModels) ModelBackupFile() ModelBackupFile {
	return &fakeModelBackupFile{}
}

func (fakeModels) ModelBackupEndpoint() ModelBackupEndpoint {
	return &fakeModelBackupEndpoint{}
}
