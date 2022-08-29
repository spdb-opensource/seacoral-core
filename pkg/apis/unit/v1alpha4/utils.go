package v1alpha4

import (
	"fmt"
	"strings"
)

func (unit *Unit) PodName() string {
	return unit.GetName()
}

func (unit *Unit) ServiceName() string {

	return "service-" + unit.Name
}

func (unit *Unit) ScriptName() string {
	return unit.Name + "-script"
}

func (u *Unit) Valid() error {
	return nil
}

func volumeName(name string) string {
	return strings.Replace(name, ".", "-", -1)
}

func GetVolumePathName(unit *Unit, volume string) string {
	return GetPersistentVolumeName(unit, volume)
}

func GetLunGroupName(unit *Unit, volume string) string {
	return GetPersistentVolumeName(unit, volume)
}

func GetPersistentVolumeClaimName(unit *Unit, volume string) string {
	return GetPersistentVolumeName(unit, volume)
}

func GetPodName(unit *Unit) string {
	return unit.GetName()
}

func GetRetainVolumeName(unit *Unit, volume string) string {
	return fmt.Sprintf("%s-%d-%s", unit.GetName(), unit.Status.RebuildStatus.RetainVolumeSuffix, volume)
}

func GetPersistentVolumeName(unit *Unit, volume string) string {
	if unit.Status.RebuildStatus != nil && unit.Status.RebuildStatus.CurVolumeSuffix != 0 {
		return fmt.Sprintf("%s-%d-%s", unit.GetName(), unit.Status.RebuildStatus.CurVolumeSuffix, volume)
	}

	return fmt.Sprintf("%s-%s", unit.GetName(), volume)
}

func GetScriptTemplateName(unit *Unit) string {
	return unit.Spec.MainContainerName + "-" + unit.Spec.MainImageVerison + "-script"
}

func GetTemplateConfigName(unit *Unit) string {
	return unit.Spec.MainContainerName + "-" + unit.Spec.MainImageVerison + "-config-template"
}

func GetUnitScriptConfigName(unit *Unit) string {
	return unit.GetName() + "-script"
}

func GetUnitConfigName(unit *Unit) string {
	return unit.GetName() + "-service-config"
}

func GetNetworkClaimName(unit *Unit) string {
	return unit.GetName()
}

func GetServiceName(unit *Unit) string {
	return unit.ServiceName()
}
