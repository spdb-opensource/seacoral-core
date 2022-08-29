package app

import (
	"context"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
)

func RegisterAppRoute(bankend appBankend, routers router.Adder) {
	r := &appRoute{
		bankend: bankend,
	}

	r.routes = []router.Route{
		router.NewPostRoute("/manager/apps", r.postApp),
		router.NewGetRoute("/manager/apps", r.listApps),
		router.NewGetRoute("/pagination/apps", r.listAppsWithPagination),
		router.NewGetRoute("/manager/apps/detail", r.listAppsDetail),
		router.NewDeleteRoute("/manager/apps/{app}", r.deleteApp),

		router.NewPutRoute("/manager/apps/{app}/arch", r.updateAppArch),
		router.NewPutRoute("/manager/apps/{app}/image", r.updateAppImage),
		router.NewPutRoute("/manager/apps/{app}/state", r.updateAppState),
		router.NewPutRoute("/manager/apps/{app}/resource/requests", r.updateAppResources),

		//set unit role
		router.NewPutRoute("/manager/apps/{app}/units/{unit}/role", r.setUnitRole),

		router.NewPutRoute("/manager/apps/{app}/units/{unit}/state", r.updateUnitState),
		router.NewPutRoute("/manager/apps/{app}/units/{unit}/rebuild", r.rebuildUnit),
		router.NewPutRoute("/manager/apps/{app}/units/{unit}/migrate", r.migrateUnit),
		router.NewPutRoute("/manager/apps/{app}/units/{unit}/restore", r.restoreUnit),
		router.NewPutRoute("/manager/apps/{app}/units/{unit}/resource/requests", r.updateUnitResources),
		router.NewPutRoute("/manager/apps/{app}/role", r.roleSwitch),

		//config
		router.NewGetRoute("/manager/apps/{app}/config", r.listConfig),
		router.NewPutRoute("/manager/apps/{app}/config", r.updateConfig),

		router.NewGetRoute("/manager/apps/{app}/database/users", r.listAppDBUser),
		router.NewGetRoute("/manager/apps/{app}/database/users/{user}", r.listAppDBSingleUser),
		router.NewPostRoute("/manager/apps/{app}/database/users", r.postAppUser),
		router.NewPutRoute("/manager/apps/{app}/database/users/pwd", r.resetAppUserPassword),
		router.NewDeleteRoute("/manager/apps/{app}/database/users/{user}", r.deleteAppDBUser),

		//privileges
		router.NewPutRoute("/manager/apps/{app}/database/users/privileges", r.updateAppUserPrivileges),

		router.NewGetRoute("/manager/apps/{app}/database/schemas", r.listAppDBSchema),
		router.NewGetRoute("/manager/apps/{app}/database/schemas/{schema}", r.listAppDBSchemaDetail),
		router.NewPostRoute("/manager/apps/{app}/database/schemas", r.postAppDBSchema),
		router.NewDeleteRoute("/manager/apps/{app}/database/schemas/{schema}", r.deleteAppDBSchema),

		//cmha topology_show
		router.NewGetRoute("/manager/apps/{app}/topology", r.getCmhaTopology),
		//cmha set replication_mode
		router.NewPutRoute("/manager/apps/{app}/replication/semi_sync", r.setCmhaReplMode),
		//cmha replication: set source
		router.NewPutRoute("/manager/apps/{app}/replication/set_source", r.setReplSource),
		//cmha set maintenance
		router.NewPutRoute("/manager/apps/{app}/maintenance", r.setCmhaMaintenance),

		//get json schema
		//router.NewGetRoute("/manager/jsonschema/{schema}", r.getJsonSchema),
	}

	routers.AddRouter(r)
}

type appBankend interface {
	AddApp(ctx context.Context, config api.AppConfig, subscriptionId string) (api.Application, error)
	ListApps(ctx context.Context, app, name, subscriptionId string, detail bool) (api.AppsResponse, error)
	ListAppsWithPagination(ctx context.Context, app, name, subscriptionId string, detail bool, pagination api.PaginationReq) (api.PaginationResp, error)
	DeleteApp(ctx context.Context, app string) (api.TaskObjectResponse, error)

	UpdateArch(ctx context.Context, app string, opts api.AppArchOptions) (api.TaskObjectResponse, error)
	UpdateState(ctx context.Context, app string, opts api.AppStateOptions) (api.TaskObjectResponse, error)
	UpdateImage(ctx context.Context, app string, opts api.AppImageOptions) (api.TaskObjectResponse, error)
	UpdateAppResourceRequests(ctx context.Context, app string, opts api.AppResourcesOptions) (api.TaskObjectResponse, error)

	//set unit role
	UnitRoleSet(ctx context.Context, app, unit string, opts api.RoleSetOptions) error

	UpdateUnitState(ctx context.Context, app, unit string, opts api.AppStateOptions) (api.TaskObjectResponse, error)
	UnitRebuild(ctx context.Context, app, unit string, opts api.UnitRebuildOptions) (api.TaskObjectResponse, error)
	UnitMigrate(ctx context.Context, app, unit string, opts api.UnitMigrateOptions) (api.TaskObjectResponse, error)
	UnitRestore(ctx context.Context, app, unit string, opts api.UnitRestoreOptions) (api.TaskObjectResponse, error)
	UpdateUnitResourceRequests(ctx context.Context, app, unit string, opts api.AppResourcesOptions) (api.TaskObjectResponse, error)
	RoleSwitch(ctx context.Context, app string, config api.UnitRoleSwitchConfig) error

	//config
	ListConfig(ctx context.Context, app string) (api.ConfigMapResponse, error)
	UpdateConfig(ctx context.Context, app string, config api.ConfigMapOptions) error

	AddAppDBUser(ctx context.Context, app string, config api.AppUserConfig) (api.TaskObjectResponse, error)
	ResetAppDBUser(ctx context.Context, app string, config api.AppUserResetConfig) error
	GetAppDBUser(ctx context.Context, appID, user, ip string) (api.DatabaseUser, error)
	ListAppDBUsers(ctx context.Context, app string) (api.AppUsersResponse, error)
	DeleteAppDBUser(ctx context.Context, app, user, ip string) error
	//privileges
	UpdateUserPrivileges(ctx context.Context, appID string, opts api.AppUserPrivilegesOptions) error

	ListAppDBSchema(ctx context.Context, app string) (api.DBSchemaResponse, error)
	ListAppDBSchemaDetail(ctx context.Context, app, schema string) (api.DBSchemaDetailResponse, error)
	AddAppDBSchema(ctx context.Context, app string, config api.AppSchemaConfig) (api.TaskObjectResponse, error)
	DeleteAppDBSchema(ctx context.Context, app, schema string) error

	GetCmhaTopology(ctx context.Context, app string) (api.CmhaTopology, error)
	SetCmhaReplMode(ctx context.Context, app string, req api.CmhaReplModeOptions) error
	SetCmhaMaintenance(ctx context.Context, app string, req api.CmhaMaintenanceOptions) error
	CmhaReplSetSource(ctx context.Context, app string, req api.CmhaReplModeOptions) error

	CheckAppAndSubscription(ctx context.Context, app, subscriptionId string) error
}

type appRoute struct {
	bankend appBankend

	routes []router.Route
}

func (ar appRoute) Routes() []router.Route {
	return ar.routes
}
