package bankend

import (
	"context"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/pkg/structs"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"k8s.io/klog/v2"
)

func (beApp *bankendApp) AddAppDBUsers(_ context.Context, id string, config []api.AppUserConfig, units []model.Unit, masterUnitName string) (api.TaskObjectResponse, error) {

	cmd := make([][]string, len(config))

	for i, configOne := range config {
		dataOne, err := encodeJson(configOne)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}
		cmd[i] = []string{
			"sh",
			shell,
			"user",
			"add",
			string(dataOne),
		}
	}

	find, _, err := beApp.doAppDBCmd(id, units, masterUnitName, cmd)

	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	if !find {
		return api.TaskObjectResponse{}, stderror.Errorf("Cannot find master pod")
	}

	return api.TaskObjectResponse{}, nil
}

func (beApp *bankendApp) AddAppDBUser(ctx context.Context, id string, config api.AppUserConfig) (api.TaskObjectResponse, error) {
	return beApp.AddAppDBUsers(ctx, id, []api.AppUserConfig{config}, nil, "")
}

func (beApp *bankendApp) GetAppDBUser(_ context.Context, appID, user, ip string) (api.DatabaseUser, error) {

	cmd := []string{
		"sh",
		shell,
		"user",
		"get",
		fmt.Sprintf(`{"name":"%s","ip":"%s"}`, user, ip),
	}

	find, res, err := beApp.doAppDBCmd(appID, nil, "", [][]string{cmd})

	if err != nil {
		return api.DatabaseUser{}, err
	}
	if !find {
		return api.DatabaseUser{}, stderror.Errorf("Cannot find master pod")
	}
	if len(res) < 1 {
		return api.DatabaseUser{}, stderror.Errorf("wrong response")
	}

	dbUser := api.DatabaseUser{}
	err = decodeJson(res[0], &dbUser)

	if err != nil {
		return api.DatabaseUser{}, err
	} else {
		return dbUser, nil
	}
}

func (beApp *bankendApp) ListAppDBUsers(_ context.Context, appID string) (api.AppUsersResponse, error) {

	cmd := []string{
		"sh",
		shell,
		"user",
		"list",
	}

	find, res, err := beApp.doAppDBCmd(appID, nil, "", [][]string{cmd})

	if err != nil {
		return api.AppUsersResponse{}, err
	}
	if !find {
		return api.AppUsersResponse{}, stderror.Errorf("Cannot find master pod")
	}
	if len(res) < 1 {
		return api.AppUsersResponse{}, stderror.Errorf("wrong response")
	}

	var users []api.DatabaseUser
	err = decodeJson(res[0], &users)

	if err != nil {
		return api.AppUsersResponse{}, err
	} else {
		return users, nil
	}
}

func (beApp *bankendApp) ResetAppDBUser(_ context.Context, id string, config api.AppUserResetConfig) error {

	data, err := encodeJson(config)
	if err != nil {
		return err
	}

	cmd := []string{
		"sh",
		shell,
		"user",
		"reset_pwd",
		string(data),
	}

	find, _, err := beApp.doAppDBCmd(id, nil, "", [][]string{cmd})

	if err != nil {
		return err
	}
	if !find {
		return stderror.Errorf("Cannot find master pod")
	}

	return nil
}

func (beApp *bankendApp) DeleteAppDBUser(_ context.Context, id, user, ip string) error {

	cmd := []string{
		"sh",
		shell,
		"user",
		"delete",
		fmt.Sprintf(`{"name":"%s","ip":"%s"}`, user, ip),
	}

	find, _, err := beApp.doAppDBCmd(id, nil, "", [][]string{cmd})

	if err != nil {
		return err
	}
	if !find {
		return stderror.Errorf("Cannot find master pod")
	}

	return nil
}

func (beApp *bankendApp) RoleSwitch(_ context.Context, id string, config api.UnitRoleSwitchConfig) error {
	app, err := beApp.m.Get(id)
	if err != nil {
		return err
	}

	masterCmd := []string{
		"sh",
		shell,
		"replication",
		"reset_master",
	}

	slaveCmd := []string{
		"sh",
		shell,
		"replication",
		"reset_slave",
	}

	masterID := ""
	masterIP := ""
	for i := range config.Units {
		if config.Units[i].Role == "master" {
			masterID = config.Units[i].ID
			break
		}
	}

	if masterID == "" {
		return fmt.Errorf("Does not find master from config")
	}

	unitList, err := beApp.listAppUnitsByType(id, app.Units, structs.MysqlServiceType, false)
	if err != nil {
		return fmt.Errorf("failed to find units in this app")
	}

	for i := range unitList {
		if unitList[i].ID == masterID {
			masterIP = unitList[i].IP.String()
			break
		}
	}

	if masterIP == "" {
		return fmt.Errorf("Does not find master from config")
	}

	var slaveip []string
	for j := range unitList {
		if unitList[j].ID != masterID {
			slaveip = append(slaveip, unitList[j].IP.String())
		}
	}

	slaveIP := api.UserAddOptions{
		IP: slaveip,
	}

	appSpec, _ := decodeAppSpec(app.Spec)
	archMode := appSpec.Database.Services.Arch.Mode

	data, err := encodeJson(api.RoleMasterOptions{ArchMode: archMode})
	if err != nil {
		return err
	}
	masterCmd = append(masterCmd, string(data))

	data, err = encodeJson(api.RoleSlaveOptions{ArchMode: archMode, MasterIP: masterIP})
	if err != nil {
		return err
	}
	slaveCmd = append(slaveCmd, string(data))

	for _, unit := range app.Units {
		if unit.ID == masterID {

			iface, err := beApp.zone.siteInterface(unit.Site)
			if err != nil {
				return err
			}

			unitInfo, err := iface.Units().Get(unit.Namespace, unit.ID)
			if err != nil {
				return err
			}

			podInfo, err := iface.Pods().Get(unit.Namespace, unitInfo.PodName())
			if err != nil {
				return err
			}

			if !podutil.IsRunningAndReady(podInfo) {
				return fmt.Errorf("The condition of pod %s is not ready.", unitInfo.PodName())
			} else {

				ok, _, err := beApp.zone.runInContainer(unit.Site, unit.Namespace, unit.ObjectName(), masterCmd)

				if !ok {
					return fmt.Errorf("Switch unit %s to master fail.", unit.ID)
				}

				if err != nil {
					klog.Error("Run scripts in unit: %s err: %s.", unit.ID, err)
				}

				userAddCmd := []string{
					"sh",
					shell,
					"replication",
					"user_add",
				}

				userAddData, err := encodeJson(slaveIP)
				if err != nil {
					return err
				}

				userAddCmd = append(userAddCmd, string(userAddData))

				ok, _, err = beApp.zone.runInContainer(unit.Site, unit.Namespace, unit.ObjectName(), userAddCmd)
				if !ok {
					return fmt.Errorf("Fail to add user.")
				}

				if err != nil {
					return fmt.Errorf("Run scripts [replication user_add] err: %s.", err)
				}

			}
		}
	}

	for _, unit := range app.Units {

		if unit.ID != masterID && unit.IsServiceType(structs.MysqlServiceType) {
			iface, err := beApp.zone.siteInterface(unit.Site)
			if err != nil {
				return err
			}

			unitInfo, err := iface.Units().Get(unit.Namespace, unit.ID)
			if err != nil {
				return err
			}

			podInfo, err := iface.Pods().Get(unit.Namespace, unitInfo.PodName())
			if err != nil {
				return err
			}

			if !podutil.IsRunningAndReady(podInfo) {
				klog.Error("The condition of pod %s is not ready.", unitInfo.PodName())
				break
			} else {
				ok, _, err := beApp.zone.runInContainer(unit.Site, unit.Namespace, unit.ObjectName(), slaveCmd)

				if !ok {
					_, err := beApp.zone.updateUnitAction(unit.Site, unit.Namespace, unit.ID, api.StateUnknown)
					if err != nil {
						return fmt.Errorf("Update unit status err: %s", err)
					}
				}

				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (beApp *bankendApp) runInUnitsTask(id string, units []model.Unit, cmd []string, interval, timeout time.Duration, try bool) error {
	done := make(map[string]struct{}, len(units))

	wt := beApp.waits.NewWaitTask(id, interval, func(err error) error {
		return err
	})

	return wt.WithTimeout(timeout, func() (bool, error) {

	loop:
		for _, unit := range units {

			if len(done) == len(units) {
				return true, nil
			}

			if _, ok := done[unit.ID]; ok {
				continue loop
			}

			ok, _, err := beApp.zone.runInContainer(unit.Site, unit.Namespace, unit.ObjectName(), cmd)
			if err != nil {
				return !try, err
			}

			if ok {
				done[unit.ID] = struct{}{}
			}

			if len(done) == len(units) {
				return true, nil
			}
		}

		return false, nil
	})
}

func (beApp *bankendApp) runInAppTask(app model.Application, cmd []string, interval, timeout time.Duration, try bool) error {
	return beApp.runInUnitsTask(app.ID, app.Units, cmd, interval, timeout, try)
}

func (beApp *bankendApp) UpdateUserPrivileges(ctx context.Context, appID string, opts api.AppUserPrivilegesOptions) error {
	app, err := beApp.m.Get(appID)
	if err != nil {
		return err
	}

	user, err := beApp.GetAppDBUser(ctx, appID, opts.Name, string(opts.IP))
	if err != nil {
		return err
	}

	updateopts := api.UpdateAppUserPrivileges{
		Name:             opts.Name,
		IP:               opts.IP,
		AddPrivileges:    []api.DatabasePrivilege{},
		RemovePrivileges: []api.DatabasePrivilege{},
	}

	if user.IP != opts.IP || user.Name != opts.Name {
		return fmt.Errorf("Doesn't find user: %s@%s, please check again.", opts.Name, opts.IP)
	}

	for _, exceptPrivileges := range opts.Privileges {

		find := false
		for _, origionPrivilege := range user.Privileges {
			if exceptPrivileges.DBName == origionPrivilege.DBName {
				find = true

				orgiontmp := map[string]string{}
				excetptmp := map[string]string{}

				adds := []string{}
				remove := []string{}

				for _, privilege := range origionPrivilege.Privileges {
					if privilege == "" {
						continue
					}

					orgiontmp[privilege] = privilege
				}
				for _, privilege := range exceptPrivileges.Privileges {
					if privilege == "" {
						continue
					}
					excetptmp[privilege] = privilege
				}

				for _, privilege := range exceptPrivileges.Privileges {
					_, ok := orgiontmp[privilege]
					if !ok {
						adds = append(adds, privilege)
					}
				}

				for _, privilege := range origionPrivilege.Privileges {
					_, ok := excetptmp[privilege]
					if !ok {
						remove = append(remove, privilege)
					}
				}

				if len(adds) > 0 {
					addPrivileges := api.DatabasePrivilege{
						DBName:     exceptPrivileges.DBName,
						Privileges: adds,
					}

					updateopts.AddPrivileges = append(updateopts.AddPrivileges, addPrivileges)
				}

				if len(remove) > 0 {
					removePrivileges := api.DatabasePrivilege{
						DBName:     exceptPrivileges.DBName,
						Privileges: remove,
					}
					updateopts.RemovePrivileges = append(updateopts.RemovePrivileges, removePrivileges)
				}
				break
			}

		}

		if !find {
			updateopts.AddPrivileges = append(updateopts.AddPrivileges, exceptPrivileges)
		}
	}

	for _, origionPrivilege := range user.Privileges {
		find := false

		for _, exceptPrivileges := range opts.Privileges {
			if origionPrivilege.DBName == exceptPrivileges.DBName {
				find = true
				break
			}
		}

		if !find {
			updateopts.RemovePrivileges = append(updateopts.RemovePrivileges, origionPrivilege)
		}

	}

	data, err := encodeJson(updateopts)
	if err != nil {
		return err
	}

	updateCmd := []string{
		"sh",
		shell,
		"user",
		"edit",
	}
	updateCmd = append(updateCmd, string(data))
	klog.Info(updateCmd)

	err = beApp.runInAppTask(app, updateCmd, time.Second*6, time.Second*10, false)
	if err != nil {
		return err
	}

	return nil
}

func validateDatabaseUser(config []api.DatabaseUser) error {
	for _, configOne := range config {
		if len(configOne.Name) == 0 {
			return stderror.New("user name is mandatory")
		}
	}
	return nil
}

func convertDatabaseUserArrayToAppUserConfigArray(config []api.DatabaseUser) []api.AppUserConfig {
	var res []api.AppUserConfig = make([]api.AppUserConfig, len(config))
	for i, configOne := range config {
		res[i] = api.AppUserConfig{
			Login:        nil,
			DatabaseUser: configOne,
		}
		// force set to mysql_native_password since others are unsupported yet
		res[i].AuthType = "mysql_native_password"
	}
	return res
}
