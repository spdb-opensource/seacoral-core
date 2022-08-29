package v1alpha4

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	// appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	netclientset "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
	networkinformers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/informers/externalversions/networking/v1alpha1"
	networklisters "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/listers/networking/v1alpha1"
	lunclientset "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	clientset "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/clientset/versioned"
	unitscheme "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/clientset/versioned/scheme"
	informers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/informers/externalversions/unit/v1alpha4"
	listers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/listers/unit/v1alpha4"
	lvmclientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
)

const (
	defaultRetries = 10

	controllerAgentName = "unit-controller"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Unit is synced
	SuccessSynced = "Synced"

	SuccessCreated = "SuccessfulCreated"
	SuccessUpdated = "SuccessfulUpdated"
	// ErrResourceExists is used as part of the Event 'reason' when a Unit fails
	// to sync.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a resource already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Unit"
	// MessageResourceSynced is the message used for an Event fired when a Unit
	// is synced successfully
	MessageResourceSynced = "Unit synced successfully"
)

// Controller is the controller implementation for Unit resources
type Controller struct {
	maxRetries int

	config *restclient.Config
	// kubeclientset is a standard kubernetes clientset
	kubeClient kubernetes.Interface
	// unit clientset is a clientset for our own API group
	clientset clientset.Interface
	netClient netclientset.Interface
	// cnfClient configclientset.Interface
	lunClient lunclientset.Interface
	lvmClient lvmclientset.Interface

	networkLister      networklisters.NetworkLister
	networkSynced      cache.InformerSynced
	networkClaimLister networklisters.NetworkClaimLister
	networkClaimSynced cache.InformerSynced

	unitLister listers.UnitLister
	unitSynced cache.InformerSynced
	unitStore  cache.Store

	podLister corelisters.PodLister
	podSynced cache.InformerSynced

	pvcLister corelisters.PersistentVolumeClaimLister
	pvcSynced cache.InformerSynced

	pvLister corelisters.PersistentVolumeLister
	pvSynced cache.InformerSynced

	serviceLister corelisters.ServiceLister
	serviceSynced cache.InformerSynced

	configMapLister corelisters.ConfigMapLister
	configMapSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	config *restclient.Config,
	kubeClient kubernetes.Interface,
	clientset clientset.Interface,
	netClient netclientset.Interface,
	lunClient lunclientset.Interface,
	lvmClient lvmclientset.Interface,
	coreInformers coreinformers.Interface,
	networkInformers networkinformers.Interface,
	informer informers.Interface) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(unitscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		maxRetries: defaultRetries,
		config:     config,
		kubeClient: kubeClient,
		clientset:  clientset,
		netClient:  netClient,
		lunClient:  lunClient,
		lvmClient:  lvmClient,
		unitStore:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 60*5*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(20), 200)},
		), "Units"),
		recorder: recorder,
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when Unit resources change
	unitInformer := informer.Units()
	unitInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueUnit,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueUnit(new)
		},
		DeleteFunc: controller.enqueueUnit,
	})
	controller.unitLister = unitInformer.Lister()
	controller.unitSynced = unitInformer.Informer().HasSynced

	podInformer := coreInformers.Pods()
	controller.podLister = podInformer.Lister()
	controller.podSynced = podInformer.Informer().HasSynced
	podInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			// AddFunc:    func(obj interface{}) { controller.handlePodObject(obj) },
			// UpdateFunc: func(oldObj, newObj interface{}) { controller.handlePodObject(newObj) },
			DeleteFunc: func(obj interface{}) { controller.handlePodObject(obj) },
		},
	)

	svcInformer := coreInformers.Services()
	controller.serviceLister = svcInformer.Lister()
	controller.serviceSynced = svcInformer.Informer().HasSynced

	pvcInformer := coreInformers.PersistentVolumeClaims()
	controller.pvcLister = pvcInformer.Lister()
	controller.pvcSynced = pvcInformer.Informer().HasSynced

	pvInformer := coreInformers.PersistentVolumes()
	controller.pvLister = pvInformer.Lister()
	controller.pvSynced = pvInformer.Informer().HasSynced

	configInformer := coreInformers.ConfigMaps()
	controller.configMapLister = configInformer.Lister()
	controller.configMapSynced = configInformer.Informer().HasSynced

	networkInformer := networkInformers.Networks()
	controller.networkLister = networkInformer.Lister()
	controller.networkSynced = networkInformer.Informer().HasSynced

	networkClaimInformer := networkInformers.NetworkClaims()
	controller.networkClaimLister = networkClaimInformer.Lister()
	controller.networkClaimSynced = networkClaimInformer.Informer().HasSynced

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (ctrl *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer ctrl.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Unit controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, ctrl.unitSynced, ctrl.podSynced, ctrl.pvcSynced, ctrl.pvSynced, ctrl.serviceSynced, ctrl.networkClaimSynced, ctrl.networkSynced, ctrl.configMapSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync units")
	}

	klog.Info("Starting Unit workers")
	// Launch two workers to process Unit resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(ctrl.runWorker, time.Second, stopCh)
	}

	// go wait.Until(c.reloadConfigFileLoop, 10*time.Minute, stopCh)

	klog.Info("Started Unit workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (ctrl *Controller) runWorker() {
	for ctrl.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (ctrl *Controller) processNextWorkItem() bool {
	obj, shutdown := ctrl.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer ctrl.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			ctrl.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Unit resource to be synced.
		if err := ctrl.syncHandler(key); err != nil {
			if ctrl.workqueue.NumRequeues(key) < ctrl.maxRetries {
				// Put the item back on the workqueue to handle any transient errors.
				ctrl.workqueue.AddRateLimited(key)

				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}

			ctrl.workqueue.Forget(key)
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		ctrl.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Unit resource
// with the current status of the resource.
func (ctrl *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Unit resource with this namespace/name
	unit, err := ctrl.unitLister.Units(namespace).Get(name)
	if err == nil {

		unit = unit.DeepCopy()
		ctrl.unitStore.Add(unit)

		err = ctrl.syncUnitHandler(unit)
		if err != nil {
			ctrl.recorder.Event(unit, corev1.EventTypeWarning, ErrResourceExists, err.Error())
		} else {
			ctrl.recorder.Event(unit, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
		}

	} else if errors.IsNotFound(err) {
		// The Unit resource may no longer exist, in which case we stop
		// processing.
		obj, exist, err := ctrl.unitStore.GetByKey(key)
		if err != nil {
			return err
		}

		if !exist {
			return nil
		}

		unit = obj.(*unitv4.Unit)
		// unit.Spec.Action.Delete = &unitv4.DeleteAction{
		// 	Storage: true,
		// }
		// err = c.removeUnit(unit.DeepCopy())
		// if err != nil {
		// 	c.recorder.Event(unit, corev1.EventTypeWarning, "Remove Unit", err.Error())
		// }
		ctrl.recorder.Event(unit, corev1.EventTypeWarning, "not find unit", "should not happen here")
		klog.Infof("%s:not find unit.should not happen here", unit.GetName())
	}

	return err
}

// enqueueUnit takes a Unit resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Unit.
func (ctrl *Controller) enqueueUnit(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	ctrl.workqueue.Add(key)
}

func (ctrl *Controller) handlePodObject(obj interface{}) {
	var object metav1.Object
	var ok bool
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
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	if pod, ok := object.(*corev1.Pod); ok && pod != nil {
		if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
			if ownerRef.Kind != unitKind.Kind {
				return
			}
		}

		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			runtime.HandleError(err)
			return
		}

		ctrl.workqueue.Add(key)
	}
	return
}
