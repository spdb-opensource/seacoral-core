package bankend

import (
	"encoding/json"
	"fmt"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	corev1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (b bankendBackup) GetNfsEndpointDeploymentStatus(siteId, endpointId string) (api.State, error) {
	iface, err := b.zone.siteInterface(siteId)
	if err != nil {
		return api.StateUnknown, err
	}

	provisionerName := fmt.Sprintf("nfs-provisioner-%s", endpointId[:8])
	deployment, err := iface.Deployments().Get(v1alpha1.DefaultVolumePathNameSpace, provisionerName, metav1.GetOptions{})
	if err != nil {
		return api.StateUnknown, err
	}

	count := 0
	if deployment != nil && deployment.Status.Conditions != nil {
		for _, cond := range deployment.Status.Conditions {
			if cond.Type == corev1.DeploymentAvailable && cond.Status == v1.ConditionTrue {
				count++
			}
			if cond.Type == corev1.DeploymentProgressing && cond.Status == v1.ConditionTrue {
				count++
			}
		}
	}
	var status api.State
	if count == 2 {
		status = api.StatePassing
	} else {
		status = api.StateCritical
	}
	return status, nil
}

func (b bankendBackup) AddNfsEndpointDeployment(siteId string, endpoint api.BackupEndpoint) error {

	iface, err := b.zone.siteInterface(endpoint.SiteId)
	if err != nil {
		return err
	}

	site, err := b.msite.Get(siteId)
	if err != nil {
		return err
	}

	tmpBytes, err := json.Marshal(endpoint.Config)
	if err != nil {
		return err
	}
	var nfsConfig api.BackupEndpointNfsConfig

	err = json.Unmarshal(tmpBytes, &nfsConfig)
	if err != nil {
		return err
	}

	provisionerName := fmt.Sprintf("nfs-provisioner-%s", endpoint.Id[:8])
	deployment := generateDeployment(site, nfsConfig, provisionerName)
	_, err = iface.Deployments().Create(v1.NamespaceDefault, deployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create deployment:[%s] err:%s", deployment.Name, err.Error())
		return err
	}

	sa := generateServiceAccount()
	_, err = iface.ServiceAccounts().Create(metav1.NamespaceDefault, sa)
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create ServiceAccounts:[%s] err:%s", sa.Name, err.Error())
		return err
	}

	cr := generateClusterRole()
	_, err = iface.ClusterRoles().Create(metav1.NamespaceDefault, cr, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create ClusterRoles:[%s] err:%s", cr.Name, err.Error())
		return err
	}

	crb := generateClusterRoleBinding()
	_, err = iface.ClusterRoleBindings().Create(metav1.NamespaceDefault, crb, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create ClusterRoleBindings:[%s] err:%s", crb.Name, err.Error())
		return err
	}

	r := generateRole()
	_, err = iface.Roles().Create(metav1.NamespaceDefault, r, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create Roles:[%s] err:%s", r.Name, err.Error())
		return err
	}

	rb := generateRoleBinding()
	_, err = iface.RoleBindings().Create(metav1.NamespaceDefault, rb, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create RoleBindings:[%s] err:%s", rb.Name, err.Error())
		return err
	}

	sc := generateStorageClass(provisionerName)
	_, err = iface.StorageClass().Create(metav1.NamespaceDefault, sc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create StorageClass:[%s] err:%s", sc.Name, err.Error())
		return err
	}

	return err
}

func (b bankendBackup) DeleteNfsEndpointDeployment(siteId, endpointId string) error {
	method := "nfs backup DELETE:"

	iface, err := b.zone.siteInterface(siteId)
	if err != nil {
		return err
	}

	provisionerName := fmt.Sprintf("nfs-provisioner-%s", endpointId[:8])

	err = iface.Deployments().Delete(metav1.NamespaceDefault, provisionerName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("%s iface.Deployments().Delete:%s ERR:[%s]", method, provisionerName, err.Error())
		return err
	}

	err = iface.StorageClass().Delete(metav1.NamespaceDefault, provisionerName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("%s iface.StorageClass().Delete:%s ERR:[%s]", method, provisionerName, err.Error())
		return err
	}

	return err
}

func generateDeployment(site model.Site, config api.BackupEndpointNfsConfig, provisionerName string) *corev1.Deployment {

	image := fmt.Sprintf("%s/k8s/k8s-staging-sig-storage/nfs-subdir-external-provisioner:v4.0.0", site.ImageRegistry)

	deployment := &corev1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      provisionerName,
			Namespace: metav1.NamespaceDefault,
			Labels:    make(map[string]string),
		},
		Spec: corev1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: make(map[string]string),
			},
			Strategy: corev1.DeploymentStrategy{
				Type: corev1.RecreateDeploymentStrategyType,
			},
			//Replicas: ,//Defaults to 1.
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: make(map[string]string),
				},
				Spec: v1.PodSpec{
					ServiceAccountName: labelNfsClientProvisioner,
					Containers: []v1.Container{
						{
							Name:  labelNfsClientProvisioner,
							Image: image,
							Env: []v1.EnvVar{
								{
									Name:  "PROVISIONER_NAME",
									Value: fmt.Sprintf("%s/%s", labelDbscaleIo, provisionerName),
								},
								{
									Name:  "NFS_SERVER",
									Value: config.NfsIP,
								},
								{
									Name:  "NFS_PATH",
									Value: config.NfsSource,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "nfs-client-root",
									MountPath: "/persistentvolumes",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "nfs-client-root",
							VolumeSource: v1.VolumeSource{
								NFS: &v1.NFSVolumeSource{
									Server: config.NfsIP,
									Path:   config.NfsSource,
								},
							},
						},
					},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "node-role.kubernetes.io/master",
												Operator: v1.NodeSelectorOpExists,
											},
										},
									},
								},
							},
						},
						PodAntiAffinity: &v1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"cluster-engine"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Tolerations: []v1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}

	deployment.Labels["app"] = labelNfsClientProvisioner
	deployment.Spec.Selector.MatchLabels["app"] = labelNfsClientProvisioner
	deployment.Spec.Template.Labels["app"] = labelNfsClientProvisioner

	return deployment
}

func generateServiceAccount() *v1.ServiceAccount {
	serviceAccount := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nfs-client-provisioner",
			Namespace: metav1.NamespaceDefault,
		},
	}

	return serviceAccount
}

func generateClusterRole() *rbacv1.ClusterRole {
	cr := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfs-client-provisioner-runner",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch", "create", "delete"},
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
			},
			{
				Verbs:     []string{"get", "list", "watch", "update"},
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
			},
			{
				Verbs:     []string{"create", "update", "patch"},
				APIGroups: []string{""},
				Resources: []string{"events"},
			},
		},
	}

	return cr
}

func generateClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "run-nfs-client-provisioner",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "nfs-client-provisioner",
				Namespace: metav1.NamespaceDefault,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "nfs-client-provisioner-runner",
		},
	}

	return crb
}

func generateRole() *rbacv1.Role {
	r := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "leader-locking-nfs-client-provisioner",
			Namespace: metav1.NamespaceDefault,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
			},
		},
	}

	return r
}

func generateRoleBinding() *rbacv1.RoleBinding {
	rb := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "leader-locking-nfs-client-provisioner",
			Namespace: metav1.NamespaceDefault,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "nfs-client-provisioner",
				Namespace: metav1.NamespaceDefault,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "leader-locking-nfs-client-provisioner",
		},
	}

	return rb
}

func generateStorageClass(provisionerName string) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: provisionerName,
		},
		Provisioner: fmt.Sprintf("%s/%s", labelDbscaleIo, provisionerName),
		Parameters:  make(map[string]string),
	}

	sc.Parameters["onDelete"] = "delete"
	sc.Parameters["archiveOnDelete"] = "false"
	sc.Parameters["pathPattern"] = "${.PVC.namespace}-${.PVC.annotations.nfs.io/directory-name}"

	return sc
}
