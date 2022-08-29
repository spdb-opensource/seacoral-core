package structs

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type ServiceCmdMap map[string]CmdMap
type CmdMap map[string][]string

// type ConfigPathMap map[string]string

const (
	SlaveRole  = "slave"
	MasterRole = "master"

	SiteCarContainerName = "sidecar"

	// EntranceScript 统一入口脚本
	//EntranceSctript = "/DBAASDAT/srvMGR/srvMGR"
	//init script entrance
	EntranceScript = "/opt/app-root/scripts/unitMGR"

	//service type
	MysqlServiceType    = "mysql"
	ProxysqlServiceType = "proxysql"
	//proxy short name
	ProxysqlServiceTypeSN = "proxy"
	CmhaServiceType       = "cmha"
	AllServiceType        = "all"
	CommonType            = "common"

	//command
	ServiceStopCmd      = "service-stop"
	ServiceStartCmd     = "service-start"
	ServiceInitStartCmd = "service-init-start"

	LinkInitCmd = "link_init"

	SideBarServiceStartCmd = "sidebar-start"
	SideBarServiceStopCmd  = "sidebar-stop"

	ConfigEffectCmd = "effect-config"
	VolumeShowCmd   = "show_volume"
	CatFileCmd      = "cat_file_cmd"

	DbListCmd                 = "db_list"
	DbGetDetailCmd            = "db_get_detail"
	DbAddCmd                  = "db_add"
	DbDeleteCmd               = "db_delete"
	DbReplicationInitCmd      = "replication_init"
	DbReplicationShowCmd      = "replication_show"
	DbReplicationGtidPurgeCmd = "replication_gtid_purge"

	DbUserAddCmd              = "db_user_add"
	DbUserGetCmd              = "db_user_get"
	DbUsersListCmd            = "db_users_list"
	DbUserDeleteCmd           = "db_user_delete"
	DbUserPwdResetCmd         = "db_user_pwd_reset"
	DbUserPrivilegesUpdateCmd = "db_user_privileges_update"

	UnitReplicationSet = "unit_replication_set"

	//cmha
	TopologyShowCmd   = "topology_show"
	ReplModeSetCmd    = "replication_mode_set"
	ReplSourceSetCmd  = "replication_source_set"
	MaintenanceSetCmd = "maintenance_set"

	ImageLatestTag = "latest"
)

var (
	DefaultserviceMap = map[string]service{}
)

type service struct {
	sourtNmae         string
	name              string
	cmdMap            map[string][]string
	defaultConfigPath string
}

func init() {
	initCommonCmd()
	initMysqlCmd()
	initcmhaCmd()
	initproxysqlCmd()
}

// 通用脚本
func initCommonCmd() {
	commonservice := service{}
	cmdMap := map[string][]string{
		ServiceInitStartCmd: {"sh", EntranceScript, "service", "init-start"},
		ServiceStartCmd:     {"sh", EntranceScript, "service", "start"},
		ServiceStopCmd:      {"sh", EntranceScript, "service", "stop"},
		ConfigEffectCmd:     {"sh", EntranceScript, "config", "effect"},
		VolumeShowCmd:       {"sh", EntranceScript, "volume", "show"},
		// link init
		LinkInitCmd: {"sh", EntranceScript, "link", "init"},

		//sidebarcontainer
		SideBarServiceStartCmd: {"sh", EntranceScript, "sidebar", "start"},
		SideBarServiceStopCmd:  {"sh", EntranceScript, "sidebar", "stop"},
	}
	commonservice.cmdMap = cmdMap
	RegisterServiceCmd(CommonType, commonservice)
}

func initcmhaCmd() {
	svc := service{
		defaultConfigPath: "/opt/app-root/configs/cmha.json",
		sourtNmae:         CmhaServiceType,
		name:              CmhaServiceType,
		cmdMap:            map[string][]string{},
	}
	cmdMap := map[string][]string{
		TopologyShowCmd:   {"sh", EntranceScript, "topology", "show"},
		ReplModeSetCmd:    {"sh", EntranceScript, "replication", "set_semi_sync"},
		MaintenanceSetCmd: {"sh", EntranceScript, "maintenance", "set"},
		ReplSourceSetCmd:  {"sh", EntranceScript, "replication", "set_source"},
	}

	svc.cmdMap = cmdMap
	RegisterServiceCmd(CmhaServiceType, svc)
}

func initproxysqlCmd() {
	svc := service{
		defaultConfigPath: "/opt/app-root/configs/proxy.cnf",
		sourtNmae:         ProxysqlServiceType,
		name:              ProxysqlServiceType,
		cmdMap:            map[string][]string{},
	}
	RegisterServiceCmd(ProxysqlServiceType, svc)
}

func initMysqlCmd() {
	svc := service{
		defaultConfigPath: "/opt/app-root/configs/my.cnf",
		sourtNmae:         MysqlServiceType,
		name:              MysqlServiceType,
	}
	cmdMap := map[string][]string{
		DbListCmd:      {"sh", EntranceScript, "database", "list"},
		DbGetDetailCmd: {"sh", EntranceScript, "database", "get"},
		DbAddCmd:       {"sh", EntranceScript, "database", "add"},
		DbDeleteCmd:    {"sh", EntranceScript, "database", "delete"},

		DbReplicationInitCmd:      {"sh", EntranceScript, "replication", "init"},
		DbReplicationShowCmd:      {"sh", EntranceScript, "replication", "show"},
		DbReplicationGtidPurgeCmd: {"sh", EntranceScript, "replication", "gtid_purge"},

		DbUserAddCmd:              {"sh", EntranceScript, "user", "add"},
		DbUserGetCmd:              {"sh", EntranceScript, "user", "get"},
		DbUsersListCmd:            {"sh", EntranceScript, "user", "list"},
		DbUserDeleteCmd:           {"sh", EntranceScript, "user", "delete"},
		DbUserPwdResetCmd:         {"sh", EntranceScript, "user", "reset_pwd"},
		DbUserPrivilegesUpdateCmd: {"sh", EntranceScript, "user", "edit"},

		UnitReplicationSet: {"sh", EntranceScript, "replication"},
	}

	svc.cmdMap = cmdMap
	RegisterServiceCmd(MysqlServiceType, svc)
}

//RegsiterServiceCmd is exported
func RegisterServiceCmd(svcType string, svc service) {
	DefaultserviceMap[svcType] = svc
}

func GetDefaultConfigPath(svcType string) (string, error) {
	svc, ok := DefaultserviceMap[svcType]
	if ok {
		return svc.defaultConfigPath, nil
	}

	return "", fmt.Errorf("configpath:not support the %s service type", svcType)
}

//GetExecCmd is exported
func GetExecCmd(svcType string, cmd string) ([]string, error) {
	klog.Infof("GetExecCmd: service type: %s, command: %s", svcType, cmd)
	if svc, ok := DefaultserviceMap[svcType]; ok {
		if cmd, ok := svc.cmdMap[cmd]; ok {
			return cmd, nil
		}
	}

	return nil, errors.Errorf("not find(support) the command(serviceType:%s ; cmd:%s)", svcType, cmd)
}

func GetShortType(svcType string) string {
	if svc, ok := DefaultserviceMap[svcType]; ok {
		return svc.sourtNmae
	}

	return "unknown"
}
