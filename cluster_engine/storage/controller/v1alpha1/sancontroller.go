package v1alpha1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	hostclientset "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
	hostinformers "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/informers/externalversions/host/v1alpha1"
	hostlisters "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/listers/host/v1alpha1"
	clientset "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	sanscheme "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned/scheme"
	informers "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/informers/externalversions/san/v1alpha1"
	listers "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/listers/san/v1alpha1"
	lvmclientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	lvminformers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions/volumepath/v1alpha1"
	lvmlisters "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/listers/volumepath/v1alpha1"
)

const (
	maxRetries = 10
	// successSynced is used as part of the Event 'reason' when a Object is synced
	successSynced = "Synced"
	failedSync    = "Failed"
	cmdFailed     = "Failed"
	// errResourceExists is used as part of the Event 'reason' when a Object fails
	// to sync due to a Deployment of the same name already existing.
	errResourceExists = "ErrResourceExists"

	// messageResourceSynced is the message used for an Event fired when a Object
	// is synced successfully
	messageResourceSynced = "synced successfully"
	// messageResourceExists is the message used for Events when a resource
	messageResourceExists     = "Resource %q already exists and is not managed by %s"
	messageResourceSyncFailed = "Resource %q sync failed,%s"
	messageResourceNotExists  = "Resource %q not exists and is not managed by %s"
	messageCmdExecFailed      = "CMD:%s,OUTPUT:%s,error:%s"
	messageActionFailed       = "Action %s failed,%s"
)

type Controller struct {
	key    string
	script string

	kubeClient kubernetes.Interface
	clientset  clientset.Interface
	lvmclient  lvmclientset.Interface

	sanLister listers.SanSystemLister
	sanSynced cache.InformerSynced

	hostClient hostclientset.Interface
	hostLister hostlisters.HostLister
	hostSynced cache.InformerSynced

	lunLister listers.LungroupLister
	lunSynced cache.InformerSynced

	//podLister corelisters.PodLister
	//podSynced cache.InformerSynced

	//pvcLister corelisters.PersistentVolumeClaimLister
	//pvcSynced cache.InformerSynced

	lvmlister lvmlisters.VolumePathLister
	lvmSynced cache.InformerSynced

	sanQueue  workqueue.RateLimitingInterface
	hostQueue workqueue.RateLimitingInterface
	lunQueue  workqueue.RateLimitingInterface

	//vpQueue   workqueue.RateLimitingInterface
	//pvcQueue  workqueue.RateLimitingInterface

	// cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)
	//pvcStore    cache.Store
	hostStore cache.Store
	//pvcPodCache *pvcPodCache // { key:claim,value:pod}

	recorder record.EventRecorder
}

func NewController(
	key, script string,
	kubeClient kubernetes.Interface,
	clientset clientset.Interface,
	hostclient hostclientset.Interface,
	lvmclient lvmclientset.Interface,
	coreInformers coreinformers.Interface,
	informers informers.Interface,
	hostInformer hostinformers.HostInformer,
	lvmInformers lvminformers.VolumePathInformer,
) *Controller {

	// Create event broadcaster
	// Add san-controller types to the default Kubernetes Scheme so Events can be
	// logged for san-controller types.
	utilruntime.Must(sanscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "san-controller"})

	ctrl := &Controller{
		key:        key,
		script:     script,
		kubeClient: kubeClient,
		clientset:  clientset,
		hostClient: hostclient,
		lvmclient:  lvmclient,
		recorder:   recorder,
		//pvcStore:    cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		hostStore: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		//pvcPodCache: newPVCPodCache(cache.DeletionHandlingMetaNamespaceKeyFunc),
		sanQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "San"),
		hostQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Hosts"),
		//pvcQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PVCs"),
		lunQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Lungroups"),
		//vpQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Volumepaths"),
	}

	/*	pvcInformer := coreInformers.PersistentVolumeClaims()
		pvcInformer.Informer().AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.pvcQueue, obj) },
				UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.pvcQueue, newObj) },
				DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.pvcQueue, obj) },
			},
		)
		ctrl.pvcLister = pvcInformer.Lister()
		ctrl.pvcSynced = pvcInformer.Informer().HasSynced

		podInformer := coreInformers.Pods()
		podInformer.Informer().AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc:    func(obj interface{}) { ctrl.podAddedDeletedUpdated(obj) },
				UpdateFunc: func(oldObj, newObj interface{}) { ctrl.podAddedDeletedUpdated(newObj) },
				DeleteFunc: func(obj interface{}) { ctrl.podAddedDeletedUpdated(obj) },
			},
		)
		ctrl.podLister = podInformer.Lister()
		ctrl.podSynced = podInformer.Informer().HasSynced*/

	sanInformer := informers.SanSystems()
	sanInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.sanQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.sanQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.sanQueue, obj) },
		},
	)
	ctrl.sanLister = sanInformer.Lister()
	ctrl.sanSynced = sanInformer.Informer().HasSynced

	hostInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.hostQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.hostQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.hostQueue, obj) },
		},
	)
	ctrl.hostLister = hostInformer.Lister()
	ctrl.hostSynced = hostInformer.Informer().HasSynced

	lunInformer := informers.Lungroups()
	lunInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(ctrl.lunQueue, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(ctrl.lunQueue, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(ctrl.lunQueue, obj) },
		},
	)
	ctrl.lunLister = lunInformer.Lister()
	ctrl.lunSynced = lunInformer.Informer().HasSynced

	lvmInformers.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ctrl.enqueueWork(ctrl.lunQueue, obj)
				//ctrl.enqueueWork(ctrl.vpQueue, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				ctrl.enqueueWork(ctrl.lunQueue, newObj)
				//ctrl.enqueueWork(ctrl.vpQueue, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				ctrl.enqueueWork(ctrl.lunQueue, obj)
				//ctrl.enqueueWork(ctrl.vpQueue, obj)
			},
		},
	)
	ctrl.lvmlister = lvmInformers.Lister()
	ctrl.lvmSynced = lvmInformers.Informer().HasSynced

	return ctrl
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

// Run starts all of this controller's control loops
func (ctrl *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	//defer ctrl.pvcQueue.ShutDown()
	defer ctrl.hostQueue.ShutDown()
	defer ctrl.sanQueue.ShutDown()
	defer ctrl.lunQueue.ShutDown()
	//defer ctrl.vpQueue.ShutDown()

	klog.Infof("Starting san controller")
	defer klog.Infof("Shutting down san controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.sanSynced, ctrl.hostSynced, ctrl.lvmSynced /*, ctrl.podSynced, ctrl.pvcSynced*/) {
		return fmt.Errorf("Unable to sync caches for storage controller")
	}

	//ctrl.initializeCache()

	for i := 0; i < threadiness; i++ {
		go wait.Until(ctrl.lungroupWorker, time.Second, stopCh)
		go wait.Until(ctrl.sanWorker, time.Second, stopCh)
		go wait.Until(ctrl.hostWorker, time.Second, stopCh)

		//go wait.Until(ctrl.lungroupWorker, time.Second, stopCh)
		//go wait.Until(ctrl.volumepathWorker, time.Second, stopCh)
	}

	go wait.Until(ctrl.syncAllSan, 60*time.Minute, stopCh)
	//go wait.Until(ctrl.claimWorker, time.Second, stopCh)

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

/*
// initializeCaches fills all controller caches with initial data from etcd in
// order to have the caches already filled when first addClaim/addVolume to
// perform initial synchronization of the controller.
func (ctrl *Controller) initializeCache() {
	claimList, err := ctrl.pvcLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("can't initialize caches: %v", err)
		return
	}
	for _, claim := range claimList {
		if err = ctrl.pvcStore.Add(claim.DeepCopy()); err != nil {
			klog.Errorf("error updating claim cache: %v", err)
		}
	}

	pods, err := ctrl.podLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("can't initialize caches: %v", err)
		return
	}
	for _, pod := range pods {
		clone := pod.DeepCopy()

		for _, v := range clone.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				ctrl.pvcPodCache.AddByKey(v.PersistentVolumeClaim.ClaimName, clone)
			}
		}
	}

	hosts, err := ctrl.hostLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("can't initialize caches: %v", err)
		return
	}
	for _, host := range hosts {
		if err = ctrl.hostStore.Add(host.DeepCopy()); err != nil {
			klog.Errorf("error updating claim cache: %v", err)
		}
	}

	klog.V(4).Infof("controller cache initialized")
}*/
