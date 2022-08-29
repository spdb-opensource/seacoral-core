package model

import "github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"

const labelEnabled = "enabled"
const labelUnschedulable = "unschedulable"

type ModelCluster interface {
	Insert(c Cluster) (string, error)
	Update(Cluster) error
	Delete(id string) error

	Get(id string) (Cluster, error)
	List(selector map[string]string) ([]Cluster, error)
}

type ModelSite interface {
	Insert(Site) (string, error)
	Update(Site) error
	Delete(id string) error

	Get(id string) (Site, error)
	List(selector map[string]string) ([]Site, error)
}

type ModelNetwork interface {
	Insert(Network) (string, error)
	Update(Network) error
	Delete(id string) error
	Get(id string) (Network, error)
	List(selector map[string]string) ([]Network, error)
}

type ModelHost interface {
	Insert(Host) (string, string, error)
	InsertHostTask(Host, string) (string, error)
	Update(h Host, storageMaxUsage *int) (string, error)
	UpdateHostTask(h *Host, tk Task) error
	Delete(id string) error
	Get(id string) (Host, error)
	GetHostBrief(id string) (HostBrief, error)
	List(selector map[string]string) ([]Host, error)
	ListUnits() ([]Unit, error)
}

type ModelImage interface {
	Insert(Image) (string, string, error)
	InsertImageTask(Image, string) (string, error)
	Update(Image) error
	UpdateImageTask(im *Image, tk Task) error
	Delete(id string) error
	Get(id string) (Image, error)
	GetLatest(type_, arch string) (Image, error)
	List(selector map[string]string) ([]Image, error)
}

type ModelTask interface {
	Insert(Task) (string, error)
	Update(Task) error
	Delete(id string) error

	Get(id string) (Task, error)
	LatestByRelateID(id string) (Task, error)
	List(selector map[string]string) ([]Task, error)
}

type ModelRemoteStorage interface {
	Insert(rs RemoteStorage) (string, string, error)
	InsertRemoteStorageTask(rs RemoteStorage, action string) (string, error)
	Update(rs RemoteStorage) (string, error)
	UpdateRemoteStorageTask(rs *RemoteStorage, tk Task) error
	Delete(id string) error

	Get(id string) (RemoteStorage, error)
	List(selector map[string]string) ([]RemoteStorage, error)

	InsertPool(pool StoragePool) (string, string, error)
	InsertPoolTask(StoragePool, string) (string, error)
	UpdatePool(pool StoragePool) error
	UpdatePoolTask(pool *StoragePool, tk Task) error
	DeletePool(id string) error

	GetPool(id string) (StoragePool, error)
	ListPools(selector map[string]string) ([]StoragePool, error)
}

type ModelApp interface {
	Insert(Application) (string, string, error)
	InsertUnits(units []Unit) error
	InsertAppTask(app Application, action string) (string, error)
	InsertUnitTask(unit Unit, action string) (string, error)
	Update(app Application, action string) (string, error)
	UpdateSpec(app, spec, action, user string, add, remove []Unit) (string, error)
	UpdateStatus(app, newStatus, targetService, user string) error
	UpdateAppTask(app *Application, tk Task) error
	Delete(id string) error
	Get(id string) (Application, error)
	List(selector map[string]string) ([]Application, error)
	ListWithPagination(selector map[string]string, pagination api.PaginationReq) ([]Application, api.PaginationResp, error)
}

type ModelBackupStrategy interface {
	InsertStrategy(bs BackupStrategy) (string, error)
	Lock(id string) (BackupStrategy, error)
	UpdateStrategy(bs BackupStrategy) error
	DeleteStrategy(id, app string) error
	GetStrategy(id string) (BackupStrategy, error)
	ListStrategy(selector map[string]string) ([]BackupStrategy, error)
}

type ModelBackupFile interface {
	InsertFile(bf BackupFile) (string, error)
	UpdateFile(bf BackupFile) error
	BackupJobDone(bf BackupFile) error
	DeleteFile(id string) error
	GetFile(id string) (BackupFile, error)
	ListFiles(selector map[string]string) ([]BackupFile, error)
}

type ModelBackupEndpoint interface {
	InsertEndpoint(be BackupEndpoint) (string, error)
	UpdateEndpoint(be BackupEndpoint) error
	DeleteEndpoint(id string) error
	GetEndpoint(id string) (BackupEndpoint, error)
	ListEndpoint(selector map[string]string) ([]BackupEndpoint, error)
}
