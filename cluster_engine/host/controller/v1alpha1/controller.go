package v1alpha1

import (
	"fmt"
	listers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/listers/unit/v1alpha4"
	"time"

	"k8s.io/klog/v2"
	// "k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	//	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corelisters "k8s.io/client-go/listers/core/v1"

	"golang.org/x/time/rate"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	unitinformers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/informers/externalversions"

	hostclientset "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
	hostScheme "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned/scheme"
	hostInformers "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/informers/externalversions"
	hostlisters "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/listers/host/v1alpha1"
)

const maxRetries = 5

// Controller is the controller implementation for host resources
type Controller struct {
	nodeName string

	kubeclientset kubernetes.Interface
	hostClientset hostclientset.Interface
	recorder      record.EventRecorder

	nodeLister corelisters.NodeLister
	nodeSynced cache.InformerSynced
	// nodeQueue  workqueue.RateLimitingInterface

	hostLister hostlisters.HostLister
	hostSynced cache.InformerSynced
	hostQueue  workqueue.RateLimitingInterface

	podLister       corelisters.PodLister
	podListerSynced cache.InformerSynced

	unitLister listers.UnitLister
	unitSynced cache.InformerSynced
}

// NewController returns a networking controller
func NewController(
	kubeclientset kubernetes.Interface,
	hostClientset hostclientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	hostInformerFactory hostInformers.SharedInformerFactory,
	unitInformerFactory unitinformers.SharedInformerFactory) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	hostScheme.AddToScheme(scheme.Scheme)
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "hostcontroller"})

	hostsInformer := hostInformerFactory.Host().V1alpha1().Hosts()
	nodesInformer := kubeInformerFactory.Core().V1().Nodes()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	unitInformer := unitInformerFactory.Unit().V1alpha4().Units()

	controller := &Controller{
		kubeclientset: kubeclientset,
		hostClientset: hostClientset,

		hostLister: hostsInformer.Lister(),
		hostSynced: hostsInformer.Informer().HasSynced,
		hostQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 60*5*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(30), 300)},
		), "hosts"),

		nodeLister: nodesInformer.Lister(),
		nodeSynced: nodesInformer.Informer().HasSynced,
		// nodeQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodes"),

		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,
		unitLister:      unitInformer.Lister(),
		unitSynced:      unitInformer.Informer().HasSynced,

		recorder: recorder,
	}

	klog.Info("Setting up event handlers")

	hostsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) { controller.enqueueWork(controller.hostQueue, obj) },
		AddFunc:    func(obj interface{}) { controller.enqueueWork(controller.hostQueue, obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { controller.enqueueWork(controller.hostQueue, newObj) },
	})

	nodesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.handleNodeObject,
		AddFunc:    controller.handleNodeObject,
		UpdateFunc: func(oldObj, newObj interface{}) { controller.handleNodeObject(newObj) },
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	// defer c.nodeQueue.ShutDown()
	defer c.hostQueue.ShutDown()

	klog.Info("Starting host controller")
	klog.Infof("Waiting for caches to sync for host controller")

	if !cache.WaitForCacheSync(stopCh, c.nodeSynced, c.hostSynced, c.podListerSynced, c.unitSynced) {
		return fmt.Errorf("Unable to sync caches for host controller")
	}

	klog.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.hostManagerWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")

	<-stopCh
	klog.Info("Shutting down workers")
	return nil
}

func (c *Controller) hostManagerWorker() {

	defer func() {
		err := recover()
		if err != nil {
			klog.V(1).Infof("hostManagerWorker panic :%s", err)
		}
	}()

	klog.V(4).Infoln("hostManagerWorker start..")

	workFunc := func() bool {

		obj, shutdown := c.hostQueue.Get()

		if shutdown {
			return true
		}

		err := func(obj interface{}) error {

			defer c.hostQueue.Done(obj)
			var key string
			var ok bool

			if key, ok = obj.(string); !ok {
				c.hostQueue.Forget(obj)
				return fmt.Errorf("expected string in hostQueue but got %#v", obj)
			}

			if err := c.hostManagerHandler(key); err != nil {

				if c.hostQueue.NumRequeues(key) < maxRetries {
					c.hostQueue.AddRateLimited(key)
					return fmt.Errorf("error hostManagerWorker %s:%s  ", key, err.Error())
				}
				c.hostQueue.Forget(obj)
				return fmt.Errorf("error hostManagerWorker '%s': %s and queue forget", key, err.Error())
			}

			c.hostQueue.Forget(obj)
			klog.Infof("Successfully hostManagerWorker '%s'", key)
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

//node相关的有更新，找到host key并放入队列
func (c *Controller) handleNodeObject(obj interface{}) {
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

	if node, ok := object.(*corev1.Node); ok && node != nil {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			klog.Errorf("failed to get key from object: %v", err)
			return
		}
		c.hostQueue.Add(objName)
		return
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
