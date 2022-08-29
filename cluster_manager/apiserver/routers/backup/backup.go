package backup

import (
	"context"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterBackupRoute(bankend backupBankend, routers router.Adder) {
	r := &backupRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewGetRoute("/manager/backup/files", r.listBackupFiles),
		router.NewDeleteRoute("/manager/backup/files", r.deleteBackupFile),

		router.NewPostRoute("/manager/backup/strategy", r.postStrategy),
		router.NewPutRoute("/manager/backup/strategy/{id}", r.updateStrategy),
		router.NewGetRoute("/manager/backup/strategy", r.listBackupStrategy),
		router.NewDeleteRoute("/manager/backup/strategy", r.deleteBackupStrategy),

		router.NewGetRoute("/manager/backup/endpoint", r.listBackupEndpoints),
		router.NewPostRoute("/manager/backup/endpoint", r.createBackupEndpoint),
		router.NewPutRoute("/manager/backup/endpoint/{id}", r.updateBackupEndpoint),
		router.NewGetRoute("/manager/backup/endpoint/{id}", r.getBackupEndpoint),
		router.NewDeleteRoute("/manager/backup/endpoint/{id}", r.deleteBackupEndpoint),
	}

	routers.AddRouter(r)
}

type backupRoute struct {
	bankend backupBankend

	routes []router.Route
}

func (br backupRoute) Routes() []router.Route {
	return br.routes
}

type backupBankend interface {
	ListBackupFiles(ctx context.Context, id, unit, app, site, user string) (api.BackupFilesResponse, error)
	DeleteBackupFile(ctx context.Context, id, app string) error

	AddBackupStrategy(ctx context.Context, config api.BackupStrategyConfig) (api.ObjectResponse, error)
	SetBackupStrategy(ctx context.Context, id string, opts api.BackupStrategyOptions) error
	ListBackupStrategy(ctx context.Context, id, unit, app string) (api.BackupStrategyResponse, error)
	DeleteBackupStrategy(ctx context.Context, id, app string) error

	AddEndpoint(ctx context.Context, config api.BackupEndpoint) (api.BackupEndpoint, error)
	UpdateEndpoint(ctx context.Context, id string, opts api.BackupEndpoint) (api.BackupEndpoint, error)
	ListEndpoint(ctx context.Context, siteId, endpointType string) ([]api.BackupEndpoint, error)
	GetEndpoint(ctx context.Context, id string) (api.BackupEndpoint, error)
	DeleteEndpoint(ctx context.Context, id string) error
}
