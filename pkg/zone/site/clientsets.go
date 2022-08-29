package site

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	execapi "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/api"

	serviceMonitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	serviceMonitor "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"

	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	network "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"

	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	san "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"

	lvmv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	lvm "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	unit "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/clientset/versioned"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	host "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
)

const defaultResync = 30 * time.Second

var Scheme = runtime.NewScheme()
var _ Interface = &clientset{}

func init() {
	v1.AddToScheme(Scheme)
	appsv1.AddToScheme(Scheme)
	unitv4.AddToScheme(Scheme)
	hostv1.AddToScheme(Scheme)
	lvmv1.AddToScheme(Scheme)
	networkv1.AddToScheme(Scheme)
	sanv1.AddToScheme(Scheme)
	serviceMonitorv1.AddToScheme(Scheme)
}

type controller func(client kubernetes.Interface, factory kubeinformers.SharedInformerFactory, stopCh <-chan struct{})

type clientset struct {
	stopCh <-chan struct{}

	lock        sync.Locker
	controllers map[string]controller

	kubeClient       kubernetes.Interface
	factory          kubeinformers.SharedInformerFactory
	event            *eventClientset
	pod              *podClientset
	podExec          *podExecClient
	imageDeployExec  *imageDeployExecClient
	hostLegalizeExec *hostLegalizeExecClient
	node             *nodeClientset
	configMap        *configMapClientset
	secret           *secretClientset
	pv               *pvClientset
	pvc              *pvcClientset
	service          *serviceClientset
	serviceAccount   *serviceAccountClientset
	set              *statefulsetClientset
	deployment       *deploymentClientset
	job              *jobClientset
	network          *networkClientset
	networkClaim     *networkClaimClientset
	system           *sansystemClientset
	host             *hostClientset
	lungroup         *lungroupClientset
	volumepath       *volumepathClientset
	unit             *unitClientset

	clusterRole        *clusterRoleClientset
	clusterRoleBinding *clusterRoleBindingClientset
	role               *roleClientset
	roleBinding        *roleBindingClientset

	serviceMonitor *serviceMonitorClientset
	storageClass   *storageClassClientset
}

func NewClientset(
	execAddr string,
	kubeClient kubernetes.Interface,
	networkClient network.Interface,
	sanClient san.Interface,
	hostClient host.Interface,
	lvmClient lvm.Interface,
	unitClient unit.Interface,
	monitorV1Client serviceMonitor.MonitoringV1Interface,
	tlsconfig *tls.Config,
	resync time.Duration,
	stopCh <-chan struct{}) *clientset {

	set := &clientset{
		lock:       new(sync.Mutex),
		kubeClient: kubeClient,
		stopCh:     stopCh,
	}

	if kubeClient != nil {
		set.event = NewEventClientset(kubeClient)

		set.pod = NewPodClientset(kubeClient)

		set.node = NewNodeClientset(kubeClient)

		set.configMap = NewConfigMapClientset(kubeClient)

		set.secret = NewSecretClientset(kubeClient)

		set.pv = NewPVClientset(kubeClient)

		set.pvc = NewPVCClientset(kubeClient)

		set.service = NewServiceClientset(kubeClient)

		set.serviceAccount = NewServiceAccountClientset(kubeClient)

		set.set = NewStatefulsetClientset(kubeClient)

		set.job = NewJobClientset(kubeClient)

		set.deployment = NewDeploymentClientset(kubeClient)

		set.clusterRole = NewClusterRoleClientset(kubeClient)

		set.clusterRoleBinding = NewClusterRoleBindingClientset(kubeClient)

		set.role = NewRoleClientset(kubeClient)

		set.roleBinding = NewRoleBindingClientset(kubeClient)

		set.storageClass = NewStorageClassClientset(kubeClient)
	}

	if execAddr != "" {
		set.podExec = NewPodExecClient(execAddr, tlsconfig)
		set.imageDeployExec = NewImageDeployExecClient(execAddr, tlsconfig)
		set.hostLegalizeExec = NewHostLegalizeExecClient(execAddr, tlsconfig)
	}

	if networkClient != nil {

		set.network = NewNetworkClientset(networkClient)

		set.networkClaim = NewNetworkClaimClientset(networkClient)
	}

	if sanClient != nil {

		set.system = NewSanSystemClientset(sanClient)

		set.lungroup = NewLungroupClientset(sanClient)
	}

	if hostClient != nil {

		set.host = NewHostClientset(hostClient)
	}

	if lvmClient != nil {
		set.volumepath = NewVolumepathClientset(lvmClient)
	}

	if unitClient != nil {

		set.unit = NewUnitClientset(unitClient)
	}

	if monitorV1Client != nil {
		set.serviceMonitor = NewServiceMonitorClientset(monitorV1Client)
	}

	return set
}

func (set *clientset) Controllers() map[string]controller {
	set.lock.Lock()

	m := set.controllers

	set.lock.Unlock()

	return m
}

func (set *clientset) RegisterController(key string, fn controller) {
	set.lock.Lock()
	defer set.lock.Unlock()

	if set.controllers == nil {
		set.controllers = make(map[string]controller)
	}

	if _, ok := set.controllers[key]; ok {
		return
	}

	if set.factory == nil {
		set.factory = kubeinformers.NewSharedInformerFactory(set.kubeClient, defaultResync)
	}

	set.controllers[key] = fn

	fn(set.kubeClient, set.factory, set.stopCh)
}

func (set *clientset) Connected() error {
	_, err := set.pod.client.Discovery().ServerVersion()

	return err
}

func (set *clientset) ServerVersion() (*version.Info, error) {
	return set.pod.client.Discovery().ServerVersion()
}
func (set *clientset) Events() EventInterface {
	return set.event
}

func (set *clientset) Pods() PodInterface {
	return set.pod
}

func (set *clientset) PodExec() PodExecInterface {
	return set.podExec
}

func (set *clientset) ImageDeployExec() ImageDeployExecInterface {
	return set.imageDeployExec
}

func (set *clientset) HostLegalizeExec() HostLegalizeInterface {
	return set.hostLegalizeExec
}

func (set *clientset) Nodes() NodeInterface {
	return set.node
}

func (set *clientset) ConfigMaps() ConfigMapInterface {
	return set.configMap
}

func (set *clientset) Secrets() SecretInterface {
	return set.secret
}

func (set *clientset) PersistentVolumes() PersistentVolumeInterface {
	return set.pv
}

func (set *clientset) PersistentVolumeClaims() PersistentVolumeClaimInterface {
	return set.pvc
}

func (set *clientset) Services() ServiceInterface {
	return set.service
}

func (set *clientset) ServiceMonitor() ServiceMonitorInterface {
	return set.serviceMonitor
}

func (set *clientset) ServiceAccounts() ServiceAccountInterface {
	return set.serviceAccount
}

func (set *clientset) StatefulSets() StatefulSetInterface {
	return set.set
}

func (set *clientset) Deployments() DeploymentInterface {
	return set.deployment
}

func (set *clientset) Jobs() JobInterface {
	return set.job
}

func (set *clientset) Networks() NetworkInterface {
	return set.network
}

func (set *clientset) NetworkClaims() NetworkClaimInterface {
	return set.networkClaim
}

func (set *clientset) SanSystems() SanSystemInterface {
	return set.system
}

func (set *clientset) Hosts() HostInterface {
	return set.host
}

func (set *clientset) Lungroups() LungroupInterface {
	return set.lungroup
}

func (set *clientset) VolumePaths() VolumePathInterface {
	return set.volumepath
}

func (set *clientset) Units() UnitInterface {
	return set.unit
}

func (set *clientset) ClusterRoles() ClusterRoleInterface {
	return set.clusterRole
}

func (set *clientset) ClusterRoleBindings() ClusterRoleBindingInterface {
	return set.clusterRoleBinding
}

func (set *clientset) Roles() RoleInterface {
	return set.role
}

func (set *clientset) RoleBindings() RoleBindingInterface {
	return set.roleBinding
}

func (set *clientset) StorageClass() StorageClassInterface {
	return set.storageClass
}

var _ EventInterface = &eventClientset{}

type eventClientset struct {
	client kubernetes.Interface
}

func NewEventClientset(client kubernetes.Interface) *eventClientset {
	return &eventClientset{
		client: client,
	}
}

func (client *eventClientset) Search(namespace string, objOrRef runtime.Object) (*v1.EventList, error) {
	return client.client.CoreV1().Events(namespace).Search(Scheme, objOrRef)
}

var _ PodInterface = &podClientset{}

type podClientset struct {
	client kubernetes.Interface
}

func NewPodClientset(client kubernetes.Interface) *podClientset {
	return &podClientset{
		client: client,
	}
}

func (client *podClientset) GetLogs(namespace, name string, options *v1.PodLogOptions) *restclient.Request {
	return client.client.CoreV1().Pods(namespace).GetLogs(name, options)
}

func (client *podClientset) Create(namespace string, pod *v1.Pod) (*v1.Pod, error) {
	return client.client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
}

func (client *podClientset) Update(namespace string, pod *v1.Pod) (*v1.Pod, error) {
	return client.client.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
}

func (client *podClientset) UpdateStatus(namespace string, pod *v1.Pod) (*v1.Pod, error) {
	return client.client.CoreV1().Pods(namespace).UpdateStatus(context.TODO(), pod, metav1.UpdateOptions{})
}

func (client *podClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().Pods(namespace).Delete(context.TODO(), name, options)
}

func (client *podClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Pod, err error) {
	return client.client.CoreV1().Pods(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *podClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.Pod, err error) {
	list, err := client.client.CoreV1().Pods(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *podClientset) Get(namespace string, name string) (*v1.Pod, error) {
	return client.client.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

type podExecClient struct {
	client execapi.ExecClient
}

func NewPodExecClient(addr string, config *tls.Config) *podExecClient {

	client := execapi.NewClient(addr, config)

	return &podExecClient{client: client}
}

func (client *podExecClient) RunInContainer(namespace, name, container string, cmd []string, stderr, stdout io.Writer) (bool, error) {
	opts := execapi.PodExecOption{
		Namespace: namespace,
		PodName:   name,
		Container: container,
		Command:   cmd,
	}
	// if stdin != nil {
	// 	data, err := ioutil.ReadAll(stdin)
	// 	if err == nil {
	// 		opts.Stdin = string(data)
	// 	}
	// }
	out, err := client.client.Exec(context.Background(), opts)
	if err != nil {
		return out.Done, err
	}

	if stderr != nil {
		stderr.Write(out.Stderr)
	}
	if stdout != nil {
		stdout.Write(out.Stdout)
	}

	if out.Errors != "" {
		return out.Done, out
	}

	return out.Done, nil
}

type imageDeployExecClient struct {
	client execapi.ExecClient
}

func NewImageDeployExecClient(addr string, config *tls.Config) *imageDeployExecClient {
	client := execapi.NewClient(addr, config)

	return &imageDeployExecClient{client: client}
}

func (client *imageDeployExecClient) DeployImage(req execapi.DeployImageOption) error {
	opts := execapi.DeployImageOption{
		ImageRegistry: req.ImageRegistry,
		ProjectName:   req.ProjectName,
		Type:          req.Type,
		Version:       req.Version,
		Arch:          req.Arch,
	}
	klog.Infof("&&&&DeployImage info: name: %s/%s/%s, version: %s arch:%s", opts.ImageRegistry, req.ProjectName, req.Type, opts.Version, opts.Arch)

	ctx := context.Background()

	resp, err := client.client.DeployImage(ctx, opts)
	klog.Infof("DeployImage: client.client.DeployImage")
	if err != nil {
		klog.Errorf("DeployImage:client.client.DeployImage ERR:%s", err)
		return err
	}

	if resp.Errors != "" {
		klog.Errorf("DeployImage resp Err: %s", err)
		return fmt.Errorf("DeployImage err: %s", resp.Outputs)
	}

	return nil
}

type hostLegalizeExecClient struct {
	client execapi.ExecClient
}

func NewHostLegalizeExecClient(addr string, config *tls.Config) *hostLegalizeExecClient {
	client := execapi.NewClient(addr, config)

	return &hostLegalizeExecClient{client: client}
}

func (client *hostLegalizeExecClient) HostLegalize(opt execapi.LegalizeHostOption) error {
	klog.Infof("HostLegalize info: ip: %s, port: %s", opt.SSHIP, opt.SSHPort)

	ctx := context.Background()

	resp, err := client.client.LegalizeHost(ctx, opt)
	if err != nil {
		klog.Errorf("HostLegalize client.client.LegalizeHost ERR: %v", err)
		return err
	}

	if resp.Errors != "" {
		klog.Errorf("HostLegalize: host:%s , ssh_port: %v, ERR: %v", opt.SSHIP, opt.SSHPort, resp.Outputs)
		return fmt.Errorf("HostLegalize: host:%s, ssh_port: %v, ERR: %v", opt.SSHIP, opt.SSHPort, resp.Outputs+resp.Errors)
	}

	return nil
}

var _ NodeInterface = &nodeClientset{}

type nodeClientset struct {
	client kubernetes.Interface
}

func NewNodeClientset(client kubernetes.Interface) *nodeClientset {
	return &nodeClientset{
		client: client,
	}
}

func (client *nodeClientset) Create(node *v1.Node) (*v1.Node, error) {
	return client.client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
}

func (client *nodeClientset) Update(node *v1.Node) (*v1.Node, error) {
	return client.client.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
}

func (client *nodeClientset) UpdateStatus(node *v1.Node) (*v1.Node, error) {
	return client.client.CoreV1().Nodes().UpdateStatus(context.TODO(), node, metav1.UpdateOptions{})
}

func (client *nodeClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().Nodes().Delete(context.TODO(), name, options)
}

func (client *nodeClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Node, err error) {
	return client.client.CoreV1().Nodes().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *nodeClientset) List(opts metav1.ListOptions) (ret []v1.Node, err error) {
	list, err := client.client.CoreV1().Nodes().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *nodeClientset) Get(name string) (*v1.Node, error) {
	return client.client.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ ConfigMapInterface = &configMapClientset{}

type configMapClientset struct {
	client kubernetes.Interface
}

func NewConfigMapClientset(client kubernetes.Interface) *configMapClientset {
	return &configMapClientset{
		client: client,
	}
}

func (client *configMapClientset) Create(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return client.client.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
}

func (client *configMapClientset) Update(namespace string, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	return client.client.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
}

func (client *configMapClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, options)
}

func (client *configMapClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ConfigMap, err error) {
	return client.client.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *configMapClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.ConfigMap, err error) {
	list, err := client.client.CoreV1().ConfigMaps(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *configMapClientset) Get(namespace string, name string) (*v1.ConfigMap, error) {
	return client.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ SecretInterface = &secretClientset{}

type secretClientset struct {
	client kubernetes.Interface
}

func NewSecretClientset(client kubernetes.Interface) *secretClientset {
	return &secretClientset{
		client: client,
	}
}

func (client *secretClientset) Create(namespace string, secret *v1.Secret) (*v1.Secret, error) {
	return client.client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
}

func (client *secretClientset) Update(namespace string, secret *v1.Secret) (*v1.Secret, error) {
	return client.client.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
}

func (client *secretClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().Secrets(namespace).Delete(context.TODO(), name, options)
}

func (client *secretClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Secret, err error) {
	return client.client.CoreV1().Secrets(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *secretClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.Secret, err error) {
	list, err := client.client.CoreV1().Secrets(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *secretClientset) Get(namespace string, name string) (*v1.Secret, error) {
	return client.client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ PersistentVolumeInterface = &pvClientset{}

type pvClientset struct {
	client kubernetes.Interface
}

func NewPVClientset(client kubernetes.Interface) *pvClientset {
	return &pvClientset{
		client: client,
	}
}

func (client *pvClientset) Create(pv *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	return client.client.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1.CreateOptions{})
}

func (client *pvClientset) Update(pv *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	return client.client.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
}

func (client *pvClientset) UpdateStatus(pv *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	return client.client.CoreV1().PersistentVolumes().UpdateStatus(context.TODO(), pv, metav1.UpdateOptions{})
}

func (client *pvClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().PersistentVolumes().Delete(context.TODO(), name, options)
}

func (client *pvClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.PersistentVolume, err error) {
	return client.client.CoreV1().PersistentVolumes().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *pvClientset) List(opts metav1.ListOptions) (ret []v1.PersistentVolume, err error) {
	list, err := client.client.CoreV1().PersistentVolumes().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *pvClientset) Get(name string) (*v1.PersistentVolume, error) {
	return client.client.CoreV1().PersistentVolumes().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ PersistentVolumeClaimInterface = &pvcClientset{}

type pvcClientset struct {
	client kubernetes.Interface
}

func NewPVCClientset(client kubernetes.Interface) *pvcClientset {
	return &pvcClientset{
		client: client,
	}
}
func (client *pvcClientset) Create(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
}

func (client *pvcClientset) Update(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
}

func (client *pvcClientset) UpdateStatus(namespace string, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).UpdateStatus(context.TODO(), pvc, metav1.UpdateOptions{})
}

func (client *pvcClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), name, options)
}

func (client *pvcClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.PersistentVolumeClaim, err error) {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *pvcClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.PersistentVolumeClaim, err error) {
	list, err := client.client.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *pvcClientset) Get(namespace string, name string) (*v1.PersistentVolumeClaim, error) {
	return client.client.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ ServiceInterface = &serviceClientset{}

type serviceClientset struct {
	client kubernetes.Interface
}

func NewServiceClientset(client kubernetes.Interface) *serviceClientset {
	return &serviceClientset{
		client: client,
	}
}

func (client *serviceClientset) Create(namespace string, service *v1.Service) (*v1.Service, error) {
	return client.client.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
}

func (client *serviceClientset) Update(namespace string, service *v1.Service) (*v1.Service, error) {
	return client.client.CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
}

func (client *serviceClientset) UpdateStatus(namespace string, service *v1.Service) (*v1.Service, error) {
	return client.client.CoreV1().Services(namespace).UpdateStatus(context.TODO(), service, metav1.UpdateOptions{})
}

func (client *serviceClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().Services(namespace).Delete(context.TODO(), name, options)
}

func (client *serviceClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Service, err error) {
	return client.client.CoreV1().Services(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *serviceClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.Service, err error) {
	list, err := client.client.CoreV1().Services(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *serviceClientset) Get(namespace string, name string) (*v1.Service, error) {
	return client.client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ ServiceMonitorInterface = &serviceMonitorClientset{}

type serviceMonitorClientset struct {
	client serviceMonitor.MonitoringV1Interface
}

func NewServiceMonitorClientset(client serviceMonitor.MonitoringV1Interface) *serviceMonitorClientset {
	return &serviceMonitorClientset{
		client: client,
	}
}

func (client *serviceMonitorClientset) Get(namespace string, name string) (*serviceMonitorv1.ServiceMonitor, error) {
	return client.client.ServiceMonitors(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (client *serviceMonitorClientset) Create(namespace string, serviceMonitor *serviceMonitorv1.ServiceMonitor, opts metav1.CreateOptions) (*serviceMonitorv1.ServiceMonitor, error) {
	return client.client.ServiceMonitors(namespace).Create(context.TODO(), serviceMonitor, metav1.CreateOptions{})
}

func (client *serviceMonitorClientset) Delete(namespace string, name string) error {
	return client.client.ServiceMonitors(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

var _ ServiceAccountInterface = &serviceAccountClientset{}

type serviceAccountClientset struct {
	client kubernetes.Interface
}

func NewServiceAccountClientset(client kubernetes.Interface) *serviceAccountClientset {
	return &serviceAccountClientset{
		client: client,
	}
}

func (client *serviceAccountClientset) Create(namespace string, serviceAccount *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	return client.client.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
}

func (client *serviceAccountClientset) Update(namespace string, serviceAccount *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	return client.client.CoreV1().ServiceAccounts(namespace).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
}

func (client *serviceAccountClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, options)
}

func (client *serviceAccountClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ServiceAccount, err error) {
	return client.client.CoreV1().ServiceAccounts(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *serviceAccountClientset) List(namespace string, opts metav1.ListOptions) (ret []v1.ServiceAccount, err error) {
	list, err := client.client.CoreV1().ServiceAccounts(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *serviceAccountClientset) Get(namespace string, name string) (*v1.ServiceAccount, error) {
	return client.client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ StatefulSetInterface = &statefulsetClientset{}

type statefulsetClientset struct {
	client kubernetes.Interface
}

func NewStatefulsetClientset(client kubernetes.Interface) *statefulsetClientset {
	return &statefulsetClientset{
		client: client,
	}
}

func (client *statefulsetClientset) Create(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return client.client.AppsV1().StatefulSets(namespace).Create(context.TODO(), set, metav1.CreateOptions{})
}

func (client *statefulsetClientset) Update(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return client.client.AppsV1().StatefulSets(namespace).Update(context.TODO(), set, metav1.UpdateOptions{})
}

func (client *statefulsetClientset) UpdateStatus(namespace string, set *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return client.client.AppsV1().StatefulSets(namespace).UpdateStatus(context.TODO(), set, metav1.UpdateOptions{})
}

func (client *statefulsetClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.AppsV1().StatefulSets(namespace).Delete(context.TODO(), name, options)
}

func (client *statefulsetClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *appsv1.StatefulSet, err error) {
	return client.client.AppsV1().StatefulSets(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *statefulsetClientset) List(namespace string, opts metav1.ListOptions) (ret []appsv1.StatefulSet, err error) {
	list, err := client.client.AppsV1().StatefulSets(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

// func (client *statefulsetClientset) GetPodStatefulSets(pod *v1.Pod) ([]*appsv1.StatefulSet, error) {
// 	return client.informer.Lister().GetPodStatefulSets(pod)
// }

func (client *statefulsetClientset) Get(namespace string, name string) (*appsv1.StatefulSet, error) {
	return client.client.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ JobInterface = &jobClientset{}

type jobClientset struct {
	client kubernetes.Interface
}

func NewJobClientset(client kubernetes.Interface) *jobClientset {
	return &jobClientset{
		client: client,
	}
}

func (client *jobClientset) Create(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return client.client.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
}

func (client *jobClientset) Delete(namespace, name string, options metav1.DeleteOptions) error {
	return client.client.BatchV1().Jobs(namespace).Delete(context.TODO(), name, options)
}

func (client *jobClientset) List(namespace string, opts metav1.ListOptions) (ret []batchv1.Job, err error) {
	list, err := client.client.BatchV1().Jobs(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *jobClientset) Get(namespace, name string) (*batchv1.Job, error) {
	return client.client.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ NetworkInterface = &networkClientset{}

type networkClientset struct {
	client network.Interface
}

func NewNetworkClientset(client network.Interface) *networkClientset {
	return &networkClientset{
		client: client,
	}
}

func (client *networkClientset) Create(network *networkv1.Network) (*networkv1.Network, error) {
	return client.client.NetworkingV1alpha1().Networks().Create(context.TODO(), network, metav1.CreateOptions{})
}

func (client *networkClientset) Update(network *networkv1.Network) (*networkv1.Network, error) {
	return client.client.NetworkingV1alpha1().Networks().Update(context.TODO(), network, metav1.UpdateOptions{})
}

func (client *networkClientset) UpdateStatus(network *networkv1.Network) (*networkv1.Network, error) {
	return client.client.NetworkingV1alpha1().Networks().UpdateStatus(context.TODO(), network, metav1.UpdateOptions{})
}

func (client *networkClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.NetworkingV1alpha1().Networks().Delete(context.TODO(), name, options)
}

func (client *networkClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.Network, err error) {
	return client.client.NetworkingV1alpha1().Networks().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *networkClientset) List(opts metav1.ListOptions) (ret []networkv1.Network, err error) {
	list, err := client.client.NetworkingV1alpha1().Networks().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *networkClientset) Get(name string) (*networkv1.Network, error) {
	return client.client.NetworkingV1alpha1().Networks().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ NetworkClaimInterface = &networkClaimClientset{}

type networkClaimClientset struct {
	client network.Interface
}

func NewNetworkClaimClientset(client network.Interface) *networkClaimClientset {
	return &networkClaimClientset{
		client: client,
	}
}

func (client *networkClaimClientset) Create(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error) {
	return client.client.NetworkingV1alpha1().NetworkClaims().Create(context.TODO(), claim, metav1.CreateOptions{})
}

func (client *networkClaimClientset) Update(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error) {
	return client.client.NetworkingV1alpha1().NetworkClaims().Update(context.TODO(), claim, metav1.UpdateOptions{})
}

func (client *networkClaimClientset) UpdateStatus(claim *networkv1.NetworkClaim) (*networkv1.NetworkClaim, error) {
	return client.client.NetworkingV1alpha1().NetworkClaims().UpdateStatus(context.TODO(), claim, metav1.UpdateOptions{})
}

func (client *networkClaimClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.NetworkingV1alpha1().NetworkClaims().Delete(context.TODO(), name, options)
}

func (client *networkClaimClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *networkv1.NetworkClaim, err error) {
	return client.client.NetworkingV1alpha1().NetworkClaims().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *networkClaimClientset) List(opts metav1.ListOptions) (ret []networkv1.NetworkClaim, err error) {
	list, err := client.client.NetworkingV1alpha1().NetworkClaims().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *networkClaimClientset) Get(name string) (*networkv1.NetworkClaim, error) {
	return client.client.NetworkingV1alpha1().NetworkClaims().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ SanSystemInterface = &sansystemClientset{}

type sansystemClientset struct {
	client san.Interface
}

func NewSanSystemClientset(client san.Interface) *sansystemClientset {

	return &sansystemClientset{
		client: client,
	}
}

func (client *sansystemClientset) Create(sys *sanv1.SanSystem) (*sanv1.SanSystem, error) {
	return client.client.SanV1alpha1().SanSystems().Create(context.TODO(), sys, metav1.CreateOptions{})
}

func (client *sansystemClientset) Update(sys *sanv1.SanSystem) (*sanv1.SanSystem, error) {
	return client.client.SanV1alpha1().SanSystems().Update(context.TODO(), sys, metav1.UpdateOptions{})
}

func (client *sansystemClientset) UpdateStatus(sys *sanv1.SanSystem) (*sanv1.SanSystem, error) {
	return client.client.SanV1alpha1().SanSystems().UpdateStatus(context.TODO(), sys, metav1.UpdateOptions{})
}

func (client *sansystemClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.SanV1alpha1().SanSystems().Delete(context.TODO(), name, options)
}

func (client *sansystemClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *sanv1.SanSystem, err error) {
	return client.client.SanV1alpha1().SanSystems().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *sansystemClientset) List(opts metav1.ListOptions) (ret []sanv1.SanSystem, err error) {
	list, err := client.client.SanV1alpha1().SanSystems().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *sansystemClientset) Get(name string) (*sanv1.SanSystem, error) {
	return client.client.SanV1alpha1().SanSystems().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ HostInterface = &hostClientset{}

type hostClientset struct {
	client host.Interface
}

func NewHostClientset(client host.Interface) *hostClientset {
	return &hostClientset{
		client: client,
	}
}

func (client *hostClientset) Create(host *hostv1.Host) (*hostv1.Host, error) {
	return client.client.HostV1alpha1().Hosts().Create(context.TODO(), host, metav1.CreateOptions{})
}

func (client *hostClientset) Update(host *hostv1.Host) (*hostv1.Host, error) {
	return client.client.HostV1alpha1().Hosts().Update(context.TODO(), host, metav1.UpdateOptions{})
}

func (client *hostClientset) UpdateStatus(host *hostv1.Host) (*hostv1.Host, error) {
	return client.client.HostV1alpha1().Hosts().UpdateStatus(context.TODO(), host, metav1.UpdateOptions{})
}

func (client *hostClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.HostV1alpha1().Hosts().Delete(context.TODO(), name, options)
}

func (client *hostClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *hostv1.Host, err error) {
	return client.client.HostV1alpha1().Hosts().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *hostClientset) List(opts metav1.ListOptions) (ret []hostv1.Host, err error) {
	list, err := client.client.HostV1alpha1().Hosts().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *hostClientset) Get(name string) (*hostv1.Host, error) {
	return client.client.HostV1alpha1().Hosts().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ LungroupInterface = &lungroupClientset{}

type lungroupClientset struct {
	client san.Interface
}

func NewLungroupClientset(client san.Interface) *lungroupClientset {
	return &lungroupClientset{
		client: client,
	}
}

func (client *lungroupClientset) Create(lg *sanv1.Lungroup) (*sanv1.Lungroup, error) {
	return client.client.SanV1alpha1().Lungroups().Create(context.TODO(), lg, metav1.CreateOptions{})
}

func (client *lungroupClientset) Update(lg *sanv1.Lungroup) (*sanv1.Lungroup, error) {
	return client.client.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
}

func (client *lungroupClientset) UpdateStatus(lg *sanv1.Lungroup) (*sanv1.Lungroup, error) {
	return client.client.SanV1alpha1().Lungroups().UpdateStatus(context.TODO(), lg, metav1.UpdateOptions{})
}

func (client *lungroupClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.SanV1alpha1().Lungroups().Delete(context.TODO(), name, options)
}

func (client *lungroupClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *sanv1.Lungroup, err error) {
	return client.client.SanV1alpha1().Lungroups().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *lungroupClientset) List(opts metav1.ListOptions) (ret []sanv1.Lungroup, err error) {
	list, err := client.client.SanV1alpha1().Lungroups().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *lungroupClientset) Get(name string) (*sanv1.Lungroup, error) {
	return client.client.SanV1alpha1().Lungroups().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ VolumePathInterface = &volumepathClientset{}

type volumepathClientset struct {
	client lvm.Interface
}

func NewVolumepathClientset(client lvm.Interface) *volumepathClientset {
	return &volumepathClientset{
		client: client,
	}
}

func (client *volumepathClientset) Create(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error) {
	return client.client.LvmV1alpha1().VolumePaths().Create(context.TODO(), vp, metav1.CreateOptions{})
}

func (client *volumepathClientset) Update(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error) {
	return client.client.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
}

func (client *volumepathClientset) UpdateStatus(vp *lvmv1.VolumePath) (*lvmv1.VolumePath, error) {
	return client.client.LvmV1alpha1().VolumePaths().UpdateStatus(context.TODO(), vp, metav1.UpdateOptions{})
}

func (client *volumepathClientset) Delete(name string, options metav1.DeleteOptions) error {
	return client.client.LvmV1alpha1().VolumePaths().Delete(context.TODO(), name, options)
}

func (client *volumepathClientset) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *lvmv1.VolumePath, err error) {
	return client.client.LvmV1alpha1().VolumePaths().Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *volumepathClientset) List(opts metav1.ListOptions) (ret []lvmv1.VolumePath, err error) {
	list, err := client.client.LvmV1alpha1().VolumePaths().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *volumepathClientset) Get(name string) (*lvmv1.VolumePath, error) {
	return client.client.LvmV1alpha1().VolumePaths().Get(context.TODO(), name, metav1.GetOptions{})
}

var _ UnitInterface = &unitClientset{}

type unitClientset struct {
	client unit.Interface
}

func NewUnitClientset(client unit.Interface) *unitClientset {
	return &unitClientset{
		client: client,
	}
}

func (client *unitClientset) Create(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error) {
	return client.client.UnitV1alpha4().Units(namespace).Create(context.TODO(), unit, metav1.CreateOptions{})
}

func (client *unitClientset) Update(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error) {
	return client.client.UnitV1alpha4().Units(namespace).Update(context.TODO(), unit, metav1.UpdateOptions{})
}

func (client *unitClientset) UpdateStatus(namespace string, unit *unitv4.Unit) (*unitv4.Unit, error) {
	return client.client.UnitV1alpha4().Units(namespace).UpdateStatus(context.TODO(), unit, metav1.UpdateOptions{})
}

func (client *unitClientset) Delete(namespace string, name string, options metav1.DeleteOptions) error {
	return client.client.UnitV1alpha4().Units(namespace).Delete(context.TODO(), name, options)
}

func (client *unitClientset) Patch(namespace string, name string, pt types.PatchType, data []byte, subresources ...string) (result *unitv4.Unit, err error) {
	return client.client.UnitV1alpha4().Units(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{}, subresources...)
}

func (client *unitClientset) List(namespace string, opts metav1.ListOptions) (ret []unitv4.Unit, err error) {
	list, err := client.client.UnitV1alpha4().Units(namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (client *unitClientset) Get(namespace string, name string) (*unitv4.Unit, error) {
	return client.client.UnitV1alpha4().Units(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

var _ DeploymentInterface = &deploymentClientset{}

type deploymentClientset struct {
	client kubernetes.Interface
}

func NewDeploymentClientset(client kubernetes.Interface) *deploymentClientset {
	return &deploymentClientset{
		client: client,
	}
}

func (client *deploymentClientset) Create(namespace string, deployment *appsv1.Deployment, opts metav1.CreateOptions) (*appsv1.Deployment, error) {
	return client.client.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, opts)
}

func (client *deploymentClientset) Get(namespace string, name string, opts metav1.GetOptions) (*appsv1.Deployment, error) {
	return client.client.AppsV1().Deployments(namespace).Get(context.TODO(), name, opts)
}

func (client *deploymentClientset) List(namespace string, opts metav1.ListOptions) (*appsv1.DeploymentList, error) {
	return client.client.AppsV1().Deployments(namespace).List(context.TODO(), opts)
}

func (client *deploymentClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.AppsV1().Deployments(namespace).Delete(context.TODO(), name, opts)
}

var _ ClusterRoleInterface = &clusterRoleClientset{}

type clusterRoleClientset struct {
	client kubernetes.Interface
}

func NewClusterRoleClientset(client kubernetes.Interface) *clusterRoleClientset {
	return &clusterRoleClientset{
		client: client,
	}
}

func (client *clusterRoleClientset) Create(namespace string, clusterRole *rbacv1.ClusterRole, opts metav1.CreateOptions) (*rbacv1.ClusterRole, error) {
	return client.client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, opts)
}

func (client *clusterRoleClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.RbacV1().ClusterRoles().Delete(context.TODO(), name, opts)
}

var _ ClusterRoleBindingInterface = &clusterRoleBindingClientset{}

type clusterRoleBindingClientset struct {
	client kubernetes.Interface
}

func NewClusterRoleBindingClientset(client kubernetes.Interface) *clusterRoleBindingClientset {
	return &clusterRoleBindingClientset{
		client: client,
	}
}

func (client *clusterRoleBindingClientset) Create(namespace string, clusterRoleBinding *rbacv1.ClusterRoleBinding, opts metav1.CreateOptions) (*rbacv1.ClusterRoleBinding, error) {
	return client.client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, opts)
}

func (client *clusterRoleBindingClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, opts)
}

var _ RoleInterface = &roleClientset{}

type roleClientset struct {
	client kubernetes.Interface
}

func NewRoleClientset(client kubernetes.Interface) *roleClientset {
	return &roleClientset{
		client: client,
	}
}

func (client *roleClientset) Create(namespace string, role *rbacv1.Role, opts metav1.CreateOptions) (*rbacv1.Role, error) {
	return client.client.RbacV1().Roles(namespace).Create(context.TODO(), role, opts)
}

func (client *roleClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.RbacV1().Roles(namespace).Delete(context.TODO(), name, opts)
}

var _ RoleBindingInterface = &roleBindingClientset{}

type roleBindingClientset struct {
	client kubernetes.Interface
}

func NewRoleBindingClientset(client kubernetes.Interface) *roleBindingClientset {
	return &roleBindingClientset{
		client: client,
	}
}

func (client *roleBindingClientset) Create(namespace string, roleBinding *rbacv1.RoleBinding, opts metav1.CreateOptions) (*rbacv1.RoleBinding, error) {
	return client.client.RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, opts)
}

func (client *roleBindingClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.RbacV1().RoleBindings(namespace).Delete(context.TODO(), name, opts)
}

var _ StorageClassInterface = &storageClassClientset{}

type storageClassClientset struct {
	client kubernetes.Interface
}

func NewStorageClassClientset(client kubernetes.Interface) *storageClassClientset {
	return &storageClassClientset{
		client: client,
	}
}

func (client *storageClassClientset) Create(namespace string, storageClass *storagev1.StorageClass, opts metav1.CreateOptions) (*storagev1.StorageClass, error) {
	return client.client.StorageV1().StorageClasses().Create(context.TODO(), storageClass, opts)
}

func (client *storageClassClientset) Delete(namespace string, name string, opts metav1.DeleteOptions) error {
	return client.client.StorageV1().StorageClasses().Delete(context.TODO(), name, opts)
}
