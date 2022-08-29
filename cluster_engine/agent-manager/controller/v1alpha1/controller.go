package v1alpha1

import (
	"fmt"
	"time"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"

	clientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	vpScheme "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned/scheme"
	vpInformers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions"
	listers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/listers/volumepath/v1alpha1"
	"golang.org/x/time/rate"
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
)

var (
	maxRetries = 5
)

type Controller struct {
	HostName    string
	ShellDir    string
	VpClientSet clientset.Interface
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	VolumePathLister listers.VolumePathLister
	VolumePathSynced cache.InformerSynced
	VpQueue          workqueue.RateLimitingInterface

	nodeLister corelisters.NodeLister
	nodeSynced cache.InformerSynced
	nodeQueue  workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

func NewController(kubeclientset kubernetes.Interface,
	vpClient clientset.Interface,
	vpInformerFactory vpInformers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	shellDir, hostname string) *Controller {
	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	vpScheme.AddToScheme(scheme.Scheme)
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})

	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: hostname + "-kubeseed"})

	vpInformer := vpInformerFactory.Lvm().V1alpha1().VolumePaths()
	nodesInformer := kubeInformerFactory.Core().V1().Nodes()

	controller := &Controller{
		kubeclientset: kubeclientset,
		VpClientSet:   vpClient,
		recorder:      recorder,

		HostName: hostname,

		ShellDir: shellDir,

		VolumePathSynced: vpInformer.Informer().HasSynced,
		VolumePathLister: vpInformer.Lister(),
		VpQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(3*time.Second, 60*5*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(20), 200)},
		), "VolumePaths"),

		nodeLister: nodesInformer.Lister(),
		nodeSynced: nodesInformer.Informer().HasSynced,
		nodeQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodes"),
	}
	klog.Info("Setting up event handlers")

	vpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueVp,
		UpdateFunc: func(old, new interface{}) {

			//			newVp := new.(*vpv1.VolumePath)
			//			oldVp := old.(*vpv1.VolumePath)

			//			if oldVp.ResourceVersion == newVp.ResourceVersion {
			//				klog.V(4).Infof("%s  :newvp ResourceVersion == oldVp.ResourceVersion(%s),forget the event.", newVp.GetName(), newVp.ResourceVersion)
			//				return
			//			}

			controller.enqueueVp(new)
		},
		DeleteFunc: controller.deleteVpObjectHandle,
	})

	nodesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) { controller.enqueueWork(controller.nodeQueue, obj) },
		AddFunc:    func(obj interface{}) { controller.enqueueWork(controller.nodeQueue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.enqueueWork(controller.nodeQueue, newObj) },
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.VpQueue.ShutDown()
	defer c.nodeQueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting seed controller")
	klog.Infof("Waiting for caches to sync for volumepaths")

	if !cache.WaitForCacheSync(stopCh, c.nodeSynced, c.VolumePathSynced) {
		return fmt.Errorf("Unable to sync caches for volumepaths,nodes")
	}

	klog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.vpsRunWorker, 5*time.Second, stopCh)
		go wait.Until(c.vpsRunWorker, 5*time.Second, stopCh)

		go wait.Until(c.nodesRunWorker, 5*time.Second, stopCh)
	}

	//go wait.Until(c.checkMount, 5*time.Second, stopCh)

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

/*
func (c *Controller) checkMount() {

	for {
		klog.V(4).Infoln("check Mount")

		vps, err := c.VolumePathLister.List(labels.Everything())
		if err != nil {
			klog.Infof("[warn] vp list fail %s", err.Error())
			time.Sleep(checkMountIntervalTime)
			continue
		}

		for _, vp := range vps {
			//check and mounter
			if vp.Status.Status == vpv1.VpBinding && vp.Status.BindingNode == c.HostName && vp.Spec.Node == c.HostName {
				cfg := generateCommonActCfg(vp)
				vpexecfile := filepath.Join(c.ShellDir, VPShellFile)
				if err := checkVP(vpexecfile, cfg); err != nil {
					klog.V(2).Infof("[warn] %s check(mount) fail :%s", vp.GetName(), err.Error())
				}
			}
		}

		time.Sleep(checkMountIntervalTime)
	}
}
*/
func (c *Controller) vpsRunWorker() {

	klog.V(4).Infoln("vpsRunWorker start..")

	workFunc := func() bool {

		obj, shutdown := c.VpQueue.Get()

		if shutdown {
			return true
		}

		err := func(obj interface{}) error {

			defer c.VpQueue.Done(obj)
			var key string
			var ok bool

			if key, ok = obj.(string); !ok {
				c.VpQueue.Forget(obj)
				return fmt.Errorf("expected string in VpQueue but got %#v", obj)
			}

			if err := c.volumePathHandler(key); err != nil {

				if c.VpQueue.NumRequeues(key) < maxRetries {
					c.VpQueue.AddRateLimited(key)
					return fmt.Errorf("error volumePathHandler %s:%s  ", key, err.Error())
				}

				c.VpQueue.Forget(obj)
				return fmt.Errorf("error volumePathHandler '%s': %s and queue forget", key, err.Error())
			}

			c.VpQueue.Forget(obj)
			klog.Infof("Successfully vg Handler '%s'", key)
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

	klog.Infoln("vpsRunWorkers worker shutting down")
	return
}

func (c *Controller) nodesRunWorker() {

	klog.V(4).Infoln("nodesRunWorker start..")

	workFunc := func() bool {

		obj, shutdown := c.nodeQueue.Get()

		if shutdown {
			return true
		}

		err := func(obj interface{}) error {

			defer c.nodeQueue.Done(obj)
			var key string
			var ok bool

			if key, ok = obj.(string); !ok {
				c.nodeQueue.Forget(obj)
				return fmt.Errorf("expected string in nodeQueue but got %#v", obj)
			}

			if err := c.SyncNodeHandle(key); err != nil {

				if c.nodeQueue.NumRequeues(key) < maxRetries {
					c.nodeQueue.AddRateLimited(key)
					return fmt.Errorf("error SyncNodeHandle %s:%s  ", key, err.Error())
				}

				c.nodeQueue.Forget(obj)
				return fmt.Errorf("error SyncNodeHandle '%s': %s and queue forget", key, err.Error())
			}

			c.nodeQueue.Forget(obj)

			klog.Infof("Successfully node Handler '%s'", key)
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

	klog.Infoln("nodesRunWorker worker shutting down")
	return
}

func (c *Controller) enqueueVp(obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	klog.V(4).Infof("enqueueVp:%s", key)

	c.VpQueue.Add(key)
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
