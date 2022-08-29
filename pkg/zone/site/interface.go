package site

import (
	"io"

	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"

	serviceMonitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	execapi "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/api"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	lvmv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
)

type Interface interface {
	Connected() error
	ServerVersion() (*version.Info, error)

	Controllers() map[string]controller
	RegisterController(key string, fn controller)

	Events() EventInterface
	Pods() PodInterface
	PodExec() PodExecInterface
	ImageDeployExec() ImageDeployExecInterface
	HostLegalizeExec() HostLegalizeInterface

	Nodes() NodeInterface
	ConfigMaps() ConfigMapInterface
	Secrets() SecretInterface
	PersistentVolumes() PersistentVolumeInterface
	PersistentVolumeClaims() PersistentVolumeClaimInterface
	Services() ServiceInterface
	ServiceAccounts() ServiceAccountInterface
	StatefulSets() StatefulSetInterface
	Deployments() DeploymentInterface
	Jobs() JobInterface
	Networks() NetworkInterface
	NetworkClaims() NetworkClaimInterface
	SanSystems() SanSystemInterface
	Hosts() HostInterface
	Lungroups() LungroupInterface
	VolumePaths() VolumePathInterface
	Units() UnitInterface

	ClusterRoles() ClusterRoleInterface
	ClusterRoleBindings() ClusterRoleBindingInterface
	Roles() RoleInterface
	RoleBindings() RoleBindingInterface
	StorageClass() StorageClassInterface

	ServiceMonitor() ServiceMonitorInterface
}

type EventInterface interface {
	Search(namespace string, objOrRef runtime.Object) (*v1.EventList, error)
}

// PodInterface has methods to work with Pod resources.
type PodInterface interface {
	GetLogs(namespace, name string, options *v1.PodLogOptions) *restclient.Request
	Create(namespace string, pod *v1.Pod) (*v1.Pod, error)
	Update(namespace string, pod *v1.Pod) (*v1.Pod, error)
	UpdateStatus(namespace string, pod *v1.Pod) (*v1.Pod, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Pod, err error)

	// List lists all Pods in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.Pod, err error)
	// Get retrieves the Pod from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.Pod, error)
}

type PodExecInterface interface {
	RunInContainer(namespace, name, container string, cmd []string, stderr, stdout io.Writer) (bool, error)
}

type ImageDeployExecInterface interface {
	DeployImage(execapi.DeployImageOption) error
}

type HostLegalizeInterface interface {
	HostLegalize(opt execapi.LegalizeHostOption) error
}

// NodeInterface has methods to work with Node resources.
type NodeInterface interface {
	Create(node *v1.Node) (*v1.Node, error)
	Update(node *v1.Node) (*v1.Node, error)
	UpdateStatus(node *v1.Node) (*v1.Node, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Node, err error)

	// List lists all Nodes in the indexer.
	List(opts metav1.ListOptions) (ret []v1.Node, err error)
	// Get retrieves the Node from the index for a given name.
	Get(name string) (*v1.Node, error)
}

// ConfigMapInterface has methods to work with ConfigMap resources.
type ConfigMapInterface interface {
	Create(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error)
	Update(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ConfigMap, err error)

	// List lists all ConfigMaps in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.ConfigMap, err error)
	// Get retrieves the ConfigMap from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.ConfigMap, error)
}

// SecretInterface has methods to work with Secret resources.
type SecretInterface interface {
	Create(namespace string, secret *v1.Secret) (*v1.Secret, error)
	Update(namespace string, secret *v1.Secret) (*v1.Secret, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Secret, err error)

	// List lists all Secrets in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.Secret, err error)
	// Get retrieves the Secret from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.Secret, error)
}

// PersistentVolumeInterface has methods to work with PersistentVolume resources.
type PersistentVolumeInterface interface {
	Create(pv *v1.PersistentVolume) (*v1.PersistentVolume, error)
	Update(pv *v1.PersistentVolume) (*v1.PersistentVolume, error)
	UpdateStatus(pv *v1.PersistentVolume) (*v1.PersistentVolume, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.PersistentVolume, err error)

	// List lists all PersistentVolumes in the indexer.
	List(opts metav1.ListOptions) (ret []v1.PersistentVolume, err error)
	// Get retrieves the PersistentVolume from the index for a given name.
	Get(name string) (*v1.PersistentVolume, error)
}

// PersistentVolumeClaimInterface has methods to work with PersistentVolumeClaim resources.
type PersistentVolumeClaimInterface interface {
	Create(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Update(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	UpdateStatus(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.PersistentVolumeClaim, err error)

	// List lists all PersistentVolumeClaims in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.PersistentVolumeClaim, err error)
	// Get retrieves the PersistentVolumeClaim from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.PersistentVolumeClaim, error)
}

// ServiceInterface has methods to work with Service resources.
type ServiceInterface interface {
	Create(namespace string, service *v1.Service) (*v1.Service, error)
	Update(namespace string, service *v1.Service) (*v1.Service, error)
	UpdateStatus(namespace string, service *v1.Service) (*v1.Service, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Service, err error)

	// List lists all Services in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.Service, err error)
	// Get retrieves the Service from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.Service, error)
}

type ServiceMonitorInterface interface {
	Get(namespace string, name string) (*serviceMonitorv1.ServiceMonitor, error)
	Create(namespace string, serviceMonitor *serviceMonitorv1.ServiceMonitor, opts metav1.CreateOptions) (*serviceMonitorv1.ServiceMonitor, error)
	Delete(namespace string, name string) error
}

// ServiceAccountInterface has methods to work with ServiceAccount resources.
type ServiceAccountInterface interface {
	Create(namespace string, serviceAccount *v1.ServiceAccount) (*v1.ServiceAccount, error)
	Update(namespace string, serviceAccount *v1.ServiceAccount) (*v1.ServiceAccount, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ServiceAccount, err error)

	// List lists all ServiceAccounts in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []v1.ServiceAccount, err error)
	// Get retrieves the ServiceAccount from the indexer for a given namespace and name.
	Get(namespace string, name string) (*v1.ServiceAccount, error)
}

// StatefulSetInterface has methods to work with StatefulSet resources.
type StatefulSetInterface interface {
	Create(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Update(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	UpdateStatus(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *appsv1.StatefulSet, err error)

	// GetPodStatefulSets(pod *v1.Pod) ([]*appsv1.StatefulSet, error)
	// List lists all StatefulSets in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []appsv1.StatefulSet, err error)
	// Get retrieves the StatefulSet from the indexer for a given namespace and name.
	Get(namespace string, name string) (*appsv1.StatefulSet, error)
}

type DeploymentInterface interface {
	Create(namespace string, deployment *appsv1.Deployment, opts metav1.CreateOptions) (*appsv1.Deployment, error)
	//Update(namespace string,deployment *appsv1.Deployment, opts metav1.UpdateOptions) (*appsv1.Deployment, error)
	//UpdateStatus(namespace string,deployment *appsv1.Deployment, opts metav1.UpdateOptions) (*appsv1.Deployment, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
	//DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(namespace string, name string, opts metav1.GetOptions) (*appsv1.Deployment, error)
	List(namespace string, opts metav1.ListOptions) (*appsv1.DeploymentList, error)
	//Watch(namespace string,opts metav1.ListOptions) (watch.Interface, error)
	//Patch(namespace string,name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *appsv1.Deployment, err error)
	//GetScale(ctx context.Context, deploymentName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	//UpdateScale(ctx context.Context, deploymentName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

// JobInterface has methods to work with job resources.
type JobInterface interface {
	Create(namespace string, job *batchv1.Job) (*batchv1.Job, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error

	// List lists all Jobs in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []batchv1.Job, err error)
	// Get retrieves the Job from the indexer for a given namespace and name.
	Get(namespace string, name string) (*batchv1.Job, error)
}

// NetworkInterface has methods to work with Network resources.
type NetworkInterface interface {
	Create(net *networkv1.Network) (*networkv1.Network, error)
	Update(net *networkv1.Network) (*networkv1.Network, error)
	UpdateStatus(net *networkv1.Network) (*networkv1.Network, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.Network, err error)

	// List lists all Networks in the indexer.
	List(opts metav1.ListOptions) (ret []networkv1.Network, err error)
	// Get retrieves the Network from the indexer for a given name.
	Get(name string) (*networkv1.Network, error)
}

// NetworkClaimInterface has methods to work with NetworkClaim resources.
type NetworkClaimInterface interface {
	Create(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error)
	Update(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error)
	UpdateStatus(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.NetworkClaim, err error)

	// List lists all NetworkClaims in the indexer.
	List(opts metav1.ListOptions) (ret []networkv1.NetworkClaim, err error)
	// Get retrieves the NetworkClaim from the indexer for a given name.
	Get(name string) (*networkv1.NetworkClaim, error)
}

// SanSystemInterface has methods to work with SanSystem resources.
type SanSystemInterface interface {
	Create(sys *sanv1.SanSystem) (*sanv1.SanSystem, error)
	Update(sys *sanv1.SanSystem) (*sanv1.SanSystem, error)
	UpdateStatus(sys *sanv1.SanSystem) (*sanv1.SanSystem, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *sanv1.SanSystem, err error)

	// List lists all SanSystems in the indexer.
	List(opts metav1.ListOptions) (ret []sanv1.SanSystem, err error)
	// Get retrieves the SanSystem from the indexer for a given name.
	Get(name string) (*sanv1.SanSystem, error)
}

// HostInterface has methods to work with Host resources.
type HostInterface interface {
	Create(host *hostv1.Host) (*hostv1.Host, error)
	Update(host *hostv1.Host) (*hostv1.Host, error)
	UpdateStatus(host *hostv1.Host) (*hostv1.Host, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *hostv1.Host, err error)

	// List lists all Hosts in the indexer.
	List(opts metav1.ListOptions) (ret []hostv1.Host, err error)
	// Get retrieves the Host from the indexer for a given name.
	Get(name string) (*hostv1.Host, error)
}

// LungroupInterface has methods to work with Lungroup resources.
type LungroupInterface interface {
	Create(lg *sanv1.Lungroup) (*sanv1.Lungroup, error)
	Update(lg *sanv1.Lungroup) (*sanv1.Lungroup, error)
	UpdateStatus(lg *sanv1.Lungroup) (*sanv1.Lungroup, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *sanv1.Lungroup, err error)

	// List lists all Lungroups in the indexer for a given namespace.
	List(opts metav1.ListOptions) (ret []sanv1.Lungroup, err error)
	// Get retrieves the Lungroup from the indexer for a given namespace and name.
	Get(name string) (*sanv1.Lungroup, error)
}

// VolumePathInterface has methods to work with VolumePath resources.
type VolumePathInterface interface {
	Create(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error)
	Update(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error)
	UpdateStatus(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error)
	Delete(name string, options metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *lvmv1.VolumePath, err error)

	// List lists all VolumePaths in the indexer for a given namespace.
	List(opts metav1.ListOptions) (ret []lvmv1.VolumePath, err error)
	// Get retrieves the VolumePath from the indexer for a given namespace and name.
	Get(name string) (*lvmv1.VolumePath, error)
}

// UnitInterface has methods to work with Unit resources.
type UnitInterface interface {
	Create(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error)
	Update(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error)
	UpdateStatus(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error)
	Delete(namespace string, name string, options metav1.DeleteOptions) error
	Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *unitv4.Unit, err error)

	// List lists all Units in the indexer for a given namespace.
	List(namespace string, opts metav1.ListOptions) (ret []unitv4.Unit, err error)
	// Get retrieves the Unit from the indexer for a given namespace and name.
	Get(namespace string, name string) (*unitv4.Unit, error)
}

type ClusterRoleInterface interface {
	Create(namespace string, clusterRole *rbacv1.ClusterRole, opts metav1.CreateOptions) (*rbacv1.ClusterRole, error)
	//Update(ctx context.Context, clusterRole *v1.ClusterRole, opts metav1.UpdateOptions) (*v1.ClusterRole, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
	//DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	//Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ClusterRole, error)
	//List(ctx context.Context, opts metav1.ListOptions) (*v1.ClusterRoleList, error)
	//Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	//Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterRole, err error)
}

type ClusterRoleBindingInterface interface {
	Create(namespace string, clusterRoleBinding *rbacv1.ClusterRoleBinding, opts metav1.CreateOptions) (*rbacv1.ClusterRoleBinding, error)
	//Update(ctx context.Context, clusterRoleBinding *v1.ClusterRoleBinding, opts metav1.UpdateOptions) (*v1.ClusterRoleBinding, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
	//DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	//Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ClusterRoleBinding, error)
	//List(ctx context.Context, opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error)
	//Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	//Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterRoleBinding, err error)
}

type RoleInterface interface {
	Create(namespace string, role *rbacv1.Role, opts metav1.CreateOptions) (*rbacv1.Role, error)
	//Update(ctx context.Context, role *v1.Role, opts metav1.UpdateOptions) (*v1.Role, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
	//DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	//Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Role, error)
	//List(ctx context.Context, opts metav1.ListOptions) (*v1.RoleList, error)
	//Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	//Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Role, err error)
}

type RoleBindingInterface interface {
	Create(namespace string, roleBinding *rbacv1.RoleBinding, opts metav1.CreateOptions) (*rbacv1.RoleBinding, error)
	//Update(ctx context.Context, roleBinding *v1.RoleBinding, opts metav1.UpdateOptions) (*v1.RoleBinding, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
	//DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	//Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.RoleBinding, error)
	//List(ctx context.Context, opts metav1.ListOptions) (*v1.RoleBindingList, error)
	//Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	//Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RoleBinding, err error)
}

type StorageClassInterface interface {
	Create(namespace string, storageClass *storagev1.StorageClass, opts metav1.CreateOptions) (*storagev1.StorageClass, error)
	Delete(namespace string, name string, opts metav1.DeleteOptions) error
}
