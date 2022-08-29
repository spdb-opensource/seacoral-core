package bankend

import (
	"context"
	"encoding/json"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"io"
	"strings"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const shell = "/opt/app-root/scripts/unitMGR"

func (beApp *bankendApp) doAppDBCmd(appID string, units []model.Unit, masterUnitName string, cmdGroup [][]string) (bool, []io.Reader, error) {
	app, err := beApp.m.Get(appID)
	if err != nil {
		return false, nil, err
	}

	appUnits := app.Units
	if units != nil {
		appUnits = units
	}

	if strings.TrimSpace(masterUnitName) == "" {
		kUnit, ok, err := beApp.getMasterK8sMysqlUnit(appID)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, err
		}

		masterUnitName = kUnit.Name
	}

	var res []io.Reader

	for _, unit := range appUnits {
		if unit.ID == masterUnitName {
			for _, cmdOne := range cmdGroup {
				ok, r, err := beApp.zone.runInContainer(unit.Site, unit.Namespace, unit.ObjectName(), cmdOne)
				if err != nil {
					return false, nil, err
				}
				if !ok {
					klog.Info("TODO, DBCH-TOREMOVE, should not happen")
				}
				res = append(res, r)
			}
			break
		}
	}

	return true, res, nil
}

func (beApp *bankendApp) ListAppDBSchema(_ context.Context, appID string) (api.DBSchemaResponse, error) {
	cmd := []string{"sh", shell, "database", "list"}
	find, res, err := beApp.doAppDBCmd(appID, nil, "", [][]string{cmd})

	if err != nil {
		return nil, err
	}
	if !find {
		return nil, stderror.Errorf("Cannot find master pod")
	}

	out := make(map[string]api.Schema)
	var errs []error

	for _, r := range res {

		var schemas []api.Schema

		err = decodeJson(r, &schemas)
		if err == nil {
			for i := range schemas {
				out[schemas[i].Name] = schemas[i]
			}
		} else {
			errs = append(errs, err)
		}
	}

	list := make([]api.Schema, 0, len(out))
	for _, val := range out {
		list = append(list, val)
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		klog.Warning(err)
	}

	return list, utilerrors.NewAggregate(errs)
}

func (beApp *bankendApp) ListAppDBSchemaDetail(_ context.Context, appID, schema string) (api.DBSchemaDetailResponse, error) {
	cmd := []string{"sh", shell, "database", "get",
		fmt.Sprintf(`{"name":"%s"}`, schema)}

	find, res, err := beApp.doAppDBCmd(appID, nil, "", [][]string{cmd})

	if err != nil {
		return api.DBSchemaDetailResponse{}, err
	}
	if !find {
		return api.DBSchemaDetailResponse{}, stderror.Errorf("Cannot find master pod")
	}
	if len(res) < 1 {
		return api.DBSchemaDetailResponse{}, stderror.Errorf("wrong response")
	}

	var errs []error

	detail := api.DBSchemaDetailResponse{}
	err = decodeJson(res[0], &detail)
	if err == nil {
		return detail, nil
	} else {
		errs = append(errs, err)
	}

	return api.DBSchemaDetailResponse{}, utilerrors.NewAggregate(errs)
}

func (beApp *bankendApp) AddAppDBSchemas(_ context.Context, id string, config []api.AppSchemaConfig, units []model.Unit, masterUnitName string) (api.TaskObjectResponse, error) {

	cmd := make([][]string, len(config))

	for i, configOne := range config {
		dataOne, err := encodeJson(configOne)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}
		cmd[i] = []string{
			"sh",
			shell,
			"database",
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

func (beApp *bankendApp) AddAppDBSchema(_ context.Context, id string, config api.AppSchemaConfig) (api.TaskObjectResponse, error) {
	return beApp.AddAppDBSchemas(context.TODO(), id, []api.AppSchemaConfig{config}, nil, "")
}

func (beApp *bankendApp) DeleteAppDBSchema(_ context.Context, id, schema string) error {

	cmd := []string{
		"sh",
		shell,
		"database",
		"delete",
		fmt.Sprintf(`{"name":"%s"}`, schema),
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

func encodeJson(obj interface{}) ([]byte, error) {
	return json.Marshal(obj)
}

func decodeJson(r io.Reader, obj interface{}) error {
	return json.NewDecoder(r).Decode(obj)
}

func validateAppSchemaConfig(config []api.AppSchemaConfig) error {
	for _, configOne := range config {
		if len(configOne.Name) == 0 {
			return stderror.New("db name is mandatory")
		}
		_, err := encodeJson(configOne)
		if err != nil {
			return err
		}
	}
	return nil
}
