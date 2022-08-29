package v1alpha1

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	//	lev1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	clientset "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
	networkingScheme "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned/scheme"
	networkingInformers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/informers/externalversions"
	listers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/listers/networking/v1alpha1"
	// utilcore "github.com/upmio/dbscale-kube/pkg/utils/core"
)

var (
	controllerAgentName = "networking-controller"
	maxRetries          = 15
)

// Controller is the controller implementation for Networking resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// networkingClientset is a clientset for our own API group
	networkingClientset clientset.Interface

	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced

	networkLister listers.NetworkLister
	networkSynced cache.InformerSynced

	networkClaimLister listers.NetworkClaimLister
	networkClaimSynced cache.InformerSynced

	netWorkqueue workqueue.RateLimitingInterface

	podWorkqueue workqueue.RateLimitingInterface

	cliamWorkqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder

	networkingMgr NetworkingMgrInterface
}

// NewController returns a networking controller
func NewController(
	kubeclientset kubernetes.Interface,
	networkingclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	networkingInformerFactory networkingInformers.SharedInformerFactory) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	networkingScheme.AddToScheme(scheme.Scheme)
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	networkClaimInformer := networkingInformerFactory.Networking().V1alpha1().NetworkClaims()
	networkInformer := networkingInformerFactory.Networking().V1alpha1().Networks()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	controller := &Controller{
		kubeclientset:       kubeclientset,
		networkingClientset: networkingclientset,

		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,

		networkLister: networkInformer.Lister(),
		networkSynced: networkInformer.Informer().HasSynced,

		networkClaimLister: networkClaimInformer.Lister(),
		networkClaimSynced: networkClaimInformer.Informer().HasSynced,

		netWorkqueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Networks"),
		cliamWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NetworkClaims"),

		recorder: recorder,
	}

	controller.networkingMgr = NewNetworkingMgr("IP networking manager", controller)

	klog.Info("Setting up event handlers")

	networkClaimInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.deleteNetworkClaimHandle,
		AddFunc:    func(obj interface{}) { controller.enqueueWork(controller.cliamWorkqueue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.enqueueWork(controller.cliamWorkqueue, newObj) },
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handlePodObject,
		DeleteFunc: controller.handlePodObject,
		//		UpdateFunc: func(old, new interface{}) {
		//			controller.handlePodObject(new)
		//		},
	})

	networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addNetworkHandle,
		DeleteFunc: controller.delNetWorkObjectHandle,
	})

	return controller

}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.netWorkqueue.ShutDown()
	defer c.cliamWorkqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting networking controller")

	klog.Infof("Waiting for caches to sync for networking controller")
	if !cache.WaitForCacheSync(stopCh, c.networkClaimSynced, c.podListerSynced, c.networkSynced) {
		return fmt.Errorf("Unable to sync caches for networking controller")
	}

	klog.Info("init NetworkingMgr and starting NetworkingMgr works")

	if err := c.networkingMgr.Init(); err != nil {
		return err
	}

	klog.Info("Starting network workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.networkClaimRunWorker, time.Second, stopCh)
	}

	//全局查看是否有冲突IP
	go wait.Until(c.syncNetworkStatusWorker, 10*time.Second, stopCh)

	klog.Info("Started network workers")

	<-stopCh
	klog.Info("Shutting down network workers")

	return nil
}

func (c *Controller) syncNetworkStatusWorker() {

	klog.Info("syncNetworkStatusWorker")

	networks, err := c.networkLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("syncNetworkStatusWorker list network fail:%s ", err.Error())
		return
	}

	for _, network := range networks {

		key := network.GetName()
		networkMgr, ok := c.networkingMgr.GetNetwork(key)
		if !ok {
			klog.Errorf("don't find  %s network from networkingMgr", key)
			continue
		}

		all, used := networkMgr.getIPCounts()
		if network.Status.AllIPCounts != all || network.Status.UsedIPCount != used {
			c.updateNetworkStatus(network, all, used, network.Status.Conflicts)
		}
	}

	//IP冲突检查
	conflictsMap := make(map[string]string)
	networkclaims, err := c.networkClaimLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("syncNetworkStatusWorker list networkclaims fail:%s ", err.Error())
		return
	}

	for _, network := range networks {

		conflicts := []string{}
		for _, networkclaim := range networkclaims {

			if networkclaim.Spec.Network != network.GetName() {
				continue
			}

			bindIP := networkclaim.Status.BindIP
			if bindIP == "" {
				continue
			}

			//ip地址冲突
			if name, ok := conflictsMap[bindIP]; ok {
				conflicts = append(conflicts, bindIP)
				klog.Errorf("find conflict ip %s(cliams:%s and %s)", bindIP, name, networkclaim.GetName())
			}

			conflictsMap[bindIP] = networkclaim.GetName()
		}

		if len(conflicts) != 0 || (len(conflicts) == 0 && len(network.Status.Conflicts) != 0) {
			c.updateNetworkStatus(network, network.Status.AllIPCounts, network.Status.UsedIPCount, conflicts)
		}
	}

	//deice冲突
	deicesmap := make(map[string]string)
	for _, networkclaim := range networkclaims {

		device := networkclaim.Status.HostDevice
		if device == "" {
			continue
		}

		useddevice := networkclaim.Status.Host + device
		if name, ok := deicesmap[useddevice]; ok {

			_err := fmt.Errorf("find conflict devices %s(cliams:%s and %s)", useddevice, name, networkclaim.GetName())
			c.recorder.Event(networkclaim, corev1.EventTypeWarning, "find conflict devices", _err.Error())
			klog.Errorf("%s", _err.Error())
		}

		deicesmap[useddevice] = networkclaim.GetName()
	}

	klog.V(5).Infof("networkCliamIP Map %s", conflictsMap)

}

//判断全局是否已分配了该IP地址
func (c *Controller) checkIPFromClaims(IP string) error {

	networkclaims, err := c.networkClaimLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("checkIPFromClaims list networkclaims fail:%s ", err.Error())
	}

	for _, networkClaim := range networkclaims {
		if networkClaim.Status.BindIP == IP {
			return fmt.Errorf("the ip(%s) had alloc for %s cliam", IP, networkClaim.GetName())
		}
	}

	return nil
}

//处理networkClaim主逻辑
func (c *Controller) networkClaimHandler(key string) error {
	klog.V(4).Infof("networkClaimHandler  key:%s", key)

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	networkClaim, err := c.networkClaimLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	network, err := c.networkLister.Get(networkClaim.Spec.Network)
	if err != nil {
		return err
	}

	var (
		uesdPod = networkClaim.Status.Used
		allocIP = networkClaim.Status.BindIP
		status  = networkClaim.Status.Status
	)

	defer func() {
		if err != nil && networkClaim.Status.BindIP == "" && allocIP != "" {
			c.networkingMgr.ReleaseRequest(networkClaim.Spec.Network, allocIP)
		}
	}()

	//是否分配IP地址
	if networkClaim.Status.BindIP == "" {
		allocIP, err = c.networkingMgr.AllocRequest(networkClaim.Spec.Network, network.Spec.DisabledIP)
		if err != nil {
			c.recorder.Event(networkClaim, corev1.EventTypeWarning, "Alloc IP fail", err.Error())
			return err
		}

		if allocIP != "" {
			if _err := c.checkIPFromClaims(allocIP); _err != nil {
				c.recorder.Event(networkClaim, corev1.EventTypeWarning, "checkIPFromClaims fail", err.Error())
				return _err
			}
		}
	}

	//get uesd pod
	if networkClaim.Status.Used == "" {
		uesd, err := c.findUsedPod(networkClaim)
		if err != nil {
			return err
		}
		if uesd != "" {
			uesdPod = uesd
			status = networkv1.Using
		} else {
			status = networkv1.Passing
		}

	} else {
		deleted, err := c.isPodDeleted(networkClaim.Status.Used)
		if err != nil {
			return err
		}
		if deleted {
			c.recorder.Eventf(networkClaim, corev1.EventTypeNormal, "not find the %s pod ,and reset status.used", networkClaim.Status.Used)
			uesdPod = ""
			status = networkv1.Passing
		}
	}

	if networkClaim.Status.Status != status ||
		networkClaim.Status.Used != uesdPod ||
		networkClaim.Status.BindIP != allocIP {
		return c.updateNetworkClaimStatus(networkClaim, allocIP, uesdPod, status)
	}

	klog.V(5).Infof("%s: claim nothing to do.", key)
	return nil
}

func (c *Controller) findUsedPod(networkClaim *networkv1.NetworkClaim) (string, error) {
	//pods, err := c.podLister.List(labels.SelectorFromSet(labels.Set{vars.LableDBScaleKey: vars.LableDBScaleValue}))
	pods, err := c.podLister.List(labels.Everything())
	if err != nil {
		return "", err
	}

	key := networkClaim.GetName()
	for _, pod := range pods {
		podkey := fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())
		nc, ok := pod.Annotations[networkv1.NetworkClaimLabelInternal]
		if ok && nc == key {
			return podkey, nil
		}

		nc, ok = pod.Annotations[networkv1.NetworkClaimLabelExternal]
		if ok && nc == key {
			return podkey, nil
		}
	}

	//not find
	klog.Warningf("%s nc:not find used pod. all (dbscale label)pods num:%d", key, len(pods))
	return "", nil
}

func (c *Controller) isPodDeleted(key string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return false, err
	}

	trytime := 3
	for i := 0; i < trytime; i++ {
		pod, err := c.podLister.Pods(namespace).Get(name)
		if err == nil && pod != nil {
			return false, nil
		}

		if !errors.IsNotFound(err) {
			return false, err
		}

		time.Sleep(time.Second)

	}

	return true, nil
}

func (c *Controller) updateNetworkStatus(network *networkv1.Network, allIPcount, usedIPcount int32, conflicts []string) error {
	toUpdate := network.DeepCopy()
	toUpdate.Status.AllIPCounts = allIPcount
	toUpdate.Status.UsedIPCount = usedIPcount

	toUpdate.Status.Conflicts = conflicts

	_, err := c.networkingClientset.NetworkingV1alpha1().Networks().UpdateStatus(context.TODO(), toUpdate, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("updateNetworkStatus %s  fail :%s,", network.GetName(), err)
		c.recorder.Event(network, corev1.EventTypeWarning, "updateNetworkStatus fail", err.Error())
		return err
	}

	return nil
}

func (c *Controller) updateNetworkClaimStatus(networkClaim *networkv1.NetworkClaim, allocIP string, uesdPod string, status networkv1.NetworkClaimStatusType) error {
	toUpdate := networkClaim.DeepCopy()

	if toUpdate.Status.BindIP == "" {
		toUpdate.Status.BindIP = allocIP
	}

	toUpdate.Status.Used = uesdPod
	toUpdate.Status.Status = status

	if uesdPod == "" || status != networkv1.Using {
		toUpdate.Status.Used = ""
		toUpdate.Status.Host = ""
		toUpdate.Status.HostDevice = ""
		toUpdate.Status.CurBandwidth = 0
	}

	_, err := c.networkingClientset.NetworkingV1alpha1().NetworkClaims().UpdateStatus(context.TODO(), toUpdate, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("updateNetworkClaimStatus  allocIP fail :%s,uesdPod:%s,status:%s,", allocIP, uesdPod, status)
		c.recorder.Eventf(networkClaim, corev1.EventTypeWarning, "UpdateStatus fail", err.Error())
		return err
	}
	c.recorder.Eventf(networkClaim, corev1.EventTypeNormal, "UpdateStatus", "UpdateStatus  ip:%s,uesdPod:%s,status:%s", allocIP, uesdPod, status)

	return nil

}

func (c *Controller) delNetWorkObjectHandle(obj interface{}) {
	klog.V(4).Infof("delNetWorkObjectHandle %v", obj)
	network, ok := obj.(*networkv1.Network)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}

		network, ok = tombstone.Obj.(*networkv1.Network)
		if !ok {
			runtime.HandleError(fmt.Errorf("Tombstone contained object that is not a Network %#v", obj))
			return
		}
	}

	c.networkingMgr.ReleaseNetwork(network)
}

func (c *Controller) deleteNetworkClaimHandle(obj interface{}) {

	klog.V(4).Infof("deleteNetworkClaimHandle %v", obj)

	networkClaim, ok := obj.(*networkv1.NetworkClaim)

	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}

		networkClaim, ok = tombstone.Obj.(*networkv1.NetworkClaim)
		if !ok {
			runtime.HandleError(fmt.Errorf("Tombstone contained object that is not a NetworkClaim %#v", obj))
			return
		}
	}

	//释放IP资源
	c.networkingMgr.ReleaseRequest(networkClaim.Spec.Network, networkClaim.Status.BindIP)
}

func (c *Controller) networkClaimRunWorker() {

	klog.Infoln("networkClaimRunWorker start..")

	workFunc := func() bool {

		obj, shutdown := c.cliamWorkqueue.Get()

		if shutdown {
			return true

		}

		err := func(obj interface{}) error {

			defer c.cliamWorkqueue.Done(obj)
			var key string
			var ok bool

			if key, ok = obj.(string); !ok {
				c.cliamWorkqueue.Forget(obj)
				return fmt.Errorf("expected string in workqueue but got %#v", obj)
			}

			if err := c.networkClaimHandler(key); err != nil {

				if c.cliamWorkqueue.NumRequeues(key) < maxRetries {
					c.cliamWorkqueue.AddRateLimited(key)
					return fmt.Errorf("error networkClaimHandler %s:%s  ", key, err.Error())
				}

				c.cliamWorkqueue.Forget(obj)
				return fmt.Errorf("error networkClaimHandler '%s': %s and queue forget", key, err.Error())
			}

			c.cliamWorkqueue.Forget(obj)
			klog.Infof("Successfully  networkClaim %s", key)
			return nil

		}(obj)

		if err != nil {
			runtime.HandleError(err)
			return false
		}

		return false
	}

	for !workFunc() {
	}

	klog.Infoln("networkClaim worker shutting down")
	return
}

func (c *Controller) addNetworkHandle(obj interface{}) {

	var err error
	var ok bool
	var object metav1.Object

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	klog.V(4).Infof("handleNetworkObject object Namespace:%s,object Name:%s", object.GetNamespace(), object.GetName())
	network, err := c.networkLister.Get(object.GetName())
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if err := c.networkingMgr.AddNetwork(network); err != nil {
		runtime.HandleError(err)
		return
	}
}

//pod相关的有更新，找到networkclaim key并放入队列
func (c *Controller) handlePodObject(obj interface{}) {
	var key string
	var ok bool
	var object metav1.Object

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	klog.V(4).Infof("handlePodObject object Namespace:%s,object Name:%s", object.GetNamespace(), object.GetName())
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.V(2).Infof(" object(%s) isn't pod", object.GetNamespace()+object.GetName())
		return
	}

	cliams := []string{}
	cname, ok := pod.Annotations[networkv1.NetworkClaimLabelInternal]
	if ok {
		cliams = append(cliams, cname)
	}

	cname, ok = pod.Annotations[networkv1.NetworkClaimLabelExternal]
	if ok {
		cliams = append(cliams, cname)
	}

	for _, claimName := range cliams {

		NetworkClaimObj, err := c.networkingClientset.NetworkingV1alpha1().NetworkClaims().Get(context.TODO(), claimName, metav1.GetOptions{})

		if err != nil || NetworkClaimObj == nil {
			return
		}

		if key, err = cache.MetaNamespaceKeyFunc(NetworkClaimObj); err != nil {
			runtime.HandleError(err)
			return
		}

		c.cliamWorkqueue.Add(key)
	}
}

// enqueueWork adds object to given work queue.
func (ctrl *Controller) enqueueWork(queue workqueue.RateLimitingInterface, obj interface{}) {
	// Beware of "xxx deleted" events
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("failed to get key from object: %v", err)
		return
	}
	klog.V(5).Infof("enqueued %q for sync", objName)
	queue.Add(objName)
}
