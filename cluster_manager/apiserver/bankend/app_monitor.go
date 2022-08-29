package bankend

import (
	"context"
	"fmt"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/vars"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	serviceMonitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

func (beApp *bankendApp) RegisterMonitor(ctx context.Context, spec api.GroupSpec, appId, appName, groupname, grouptype string) error {
	_, err := beApp.m.Get(appId)
	if err != nil {
		return fmt.Errorf("RegisterMonitor: get service in db err: %s", err)
	}

	return beApp.serviceMonitorCreateWork(spec, appId, appName, groupname, grouptype)
}

func (beApp *bankendApp) UnregisterMonitor(ctx context.Context, appName string) error {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	for _, imageType := range []string{"mysql", "proxysql"} {
		serviceName := fmt.Sprintf("%s-%s-exporter-svc", appName, imageType)
		serviceMonitorName := fmt.Sprintf("%s-%s-exporter-svcmon", appName, imageType)

		err := iface.Services().Delete(metav1.NamespaceDefault, serviceName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete k8s service err:%s", err)
		}

		err = iface.ServiceMonitor().Delete(metav1.NamespaceSystem, serviceMonitorName)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete k8s service monitor err:%s", err)
		}
	}

	return nil
}

func (beApp *bankendApp) serviceMonitorCreateWork(spec api.GroupSpec, appId, appName, groupname, grouptype string) error {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		klog.Errorf("serviceMonitorCreateWork get site interface err:%s", err.Error())
		return err
	}

	mi, err := beApp.images.Get(spec.Image.ID)
	if err != nil {
		klog.Errorf("serviceMonitorCreateWork get image in db err:%s", err.Error())
		return err
	}

	if mi.ExporterPort == 0 {
		klog.Infof("serviceMonitorCreateWork image: %s ExportPort = 0, the service not support monitor")
		return nil
	}

	mysqlExporterPort := mi.ExporterPort
	serviceName := fmt.Sprintf("%s-%s-exporter-svc", appName, mi.Type)
	serviceMonitorName := fmt.Sprintf("%s-%s-exporter-svcmon", appName, mi.Type)

	_, kserr := iface.Services().Get(metav1.NamespaceDefault, serviceName)
	if kserr != nil && !errors.IsNotFound(kserr) {
		klog.Errorf("serviceMonitorCreateWork get k8s service err:%s", kserr.Error())
		return err
	}
	_, ksmerr := iface.ServiceMonitor().Get(metav1.NamespaceSystem, serviceMonitorName)
	if ksmerr != nil && !errors.IsNotFound(ksmerr) {
		klog.Errorf("serviceMonitorCreateWork get k8s service monitor err:%s", ksmerr.Error())
		return err
	}

	if !errors.IsNotFound(kserr) && !errors.IsNotFound(ksmerr) {
		klog.Infof("serviceMonitorCreateWork:service: %s already registered monitor", appName)
		return nil
	}

	if errors.IsNotFound(kserr) {
		k8sService := generateK8sService(serviceName, mi.Type, appId, appName, groupname, grouptype, mysqlExporterPort)
		_, err := iface.Services().Create(metav1.NamespaceDefault, &k8sService)
		if err != nil {
			klog.Errorf("serviceMonitorCreateWork create k8s service err:%s", err.Error())
			return err
		}
	}

	if errors.IsNotFound(ksmerr) {
		k8sServiceM := generateK8sServiceMonitor(serviceMonitorName, mi.Type, appId, appName, groupname, grouptype)
		_, err := iface.ServiceMonitor().Create(metav1.NamespaceSystem, &k8sServiceM, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("serviceMonitorCreateWork create k8s service monitor err:%s", err.Error())
			return err
		}
	}

	return nil
}

func generateK8sService(ksn, imageType, appId, appName, groupname, grouptype string, targetPort int) corev1.Service {
	s := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ksn,
			Namespace: metav1.NamespaceDefault,
			Labels:    make(map[string]string),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       fmt.Sprintf("%s-exporter", imageType),
					Port:       int32(targetPort),
					TargetPort: intstr.FromInt(targetPort),
				},
			},
			Selector:        make(map[string]string),
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: "None",
		},
	}

	s.Labels[vars.LabelDBScaleKey] = vars.LabelDBScaleValue
	s.Labels[v1alpha4.LabelGroup] = appName
	s.Labels[labelGroupName] = groupname
	s.Labels[labelGroupType] = grouptype
	s.Labels[labelAppID] = appId
	s.Labels[labelServiceName] = appId
	s.Labels[labelServiceImageType] = imageType

	s.Spec.Selector[vars.LabelDBScaleKey] = vars.LabelDBScaleValue
	s.Spec.Selector[v1alpha4.LabelGroup] = appName
	s.Spec.Selector[labelGroupName] = groupname
	s.Spec.Selector[labelGroupType] = grouptype
	s.Spec.Selector[labelAppID] = appId
	s.Spec.Selector[labelServiceImageType] = imageType

	return s
}

func generateK8sServiceMonitor(ksmn, imageType, appId, appName, groupname, grouptype string) serviceMonitorv1.ServiceMonitor {
	sm := serviceMonitorv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ksmn,
			Namespace: metav1.NamespaceSystem,
			Labels:    make(map[string]string),
		},
		Spec: serviceMonitorv1.ServiceMonitorSpec{
			Endpoints: []serviceMonitorv1.Endpoint{
				{
					Port:          fmt.Sprintf("%s-exporter", imageType),
					Path:          "/metrics",
					ScrapeTimeout: "10s",
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: make(map[string]string),
			},
			NamespaceSelector: serviceMonitorv1.NamespaceSelector{
				Any: true,
			},
		},
	}

	sm.Labels[labelRelease] = PrometheusNameSpace
	sm.Labels[vars.LabelDBScaleKey] = vars.LabelDBScaleValue
	sm.Labels[v1alpha4.LabelGroup] = appName
	sm.Labels[labelGroupName] = groupname
	sm.Labels[labelGroupType] = grouptype
	sm.Labels[labelAppID] = appId
	sm.Labels[labelServiceName] = appId
	sm.Labels[labelServiceImageType] = imageType

	sm.Spec.Selector.MatchLabels[vars.LabelDBScaleKey] = vars.LabelDBScaleValue
	sm.Spec.Selector.MatchLabels[v1alpha4.LabelGroup] = appName
	sm.Spec.Selector.MatchLabels[labelGroupName] = groupname
	sm.Spec.Selector.MatchLabels[labelGroupType] = grouptype
	sm.Spec.Selector.MatchLabels[labelAppID] = appId
	sm.Spec.Selector.MatchLabels[labelServiceImageType] = imageType

	return sm
}
