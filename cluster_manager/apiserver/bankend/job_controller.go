package bankend

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	// TODO: delete commented import
	// lijj32: as we use cache.DeletionHandlingMetaNamespaceKeyFunc() instead of controller.KeyFunc()
	// we don't need to import github.com/kubernetes/kubernetes/pkg/controller anymore.
	// "github.com/kubernetes/kubernetes/pkg/controller"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
)

var (
	// DefaultJobBackOff is the max backoff period, exported for the e2e test
	DefaultJobBackOff = 10 * time.Second
	// MaxJobBackOff is the max backoff period, exported for the e2e test
	MaxJobBackOff = 360 * time.Second
)

type jobController struct {
	mbf modelBackupFile

	queue workqueue.RateLimitingInterface

	jobLister      listers.JobLister
	jobStoreSynced cache.InformerSynced

	kubeClient kubernetes.Interface
	recorder   record.EventRecorder
}

func (jm *jobController) Run(client kubernetes.Interface, f kubeinformers.SharedInformerFactory, stopCh <-chan struct{}) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})
	jm.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "CM-job-controller"})

	jm.queue = workqueue.NewNamedRateLimitingQueue(
		workqueue.NewItemExponentialFailureRateLimiter(DefaultJobBackOff, MaxJobBackOff), "job")

	informer := f.Batch().V1().Jobs()
	jobInformer := informer.Informer()

	jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			jm.enqueueController(obj, true)
		},
		UpdateFunc: func(old, new interface{}) {
			jm.updateJob(old, new)
		},
		DeleteFunc: func(obj interface{}) {
			jm.enqueueController(obj, true)
		},
	})

	jm.jobStoreSynced = jobInformer.HasSynced
	jm.jobLister = informer.Lister()
	jm.kubeClient = client

	go jm.run(1, stopCh)

	go jobInformer.Run(stopCh)
}

// Run the main goroutine responsible for watching and syncing jobs.
func (jm *jobController) run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer jm.queue.ShutDown()

	klog.Infof("Starting job controller")
	defer klog.Infof("Shutting down job controller")

	if !cache.WaitForNamedCacheSync("job", stopCh, jm.jobStoreSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(jm.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (jm *jobController) worker() {
	for jm.processNextWorkItem() {
	}
}

func (jm *jobController) processNextWorkItem() bool {
	key, quit := jm.queue.Get()
	if quit {
		return false
	}
	defer jm.queue.Done(key)

	forget, err := jm.syncJob(key.(string))
	if err == nil {
		if forget {
			jm.queue.Forget(key)
		}
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Error syncing job: %v", err))
	jm.queue.AddRateLimited(key)

	return true
}

func (jm *jobController) updateJob(old, cur interface{}) {
	oldJob := old.(*batchv1.Job)
	curJob := cur.(*batchv1.Job)

	// never return error
	// lijj32: change controller.KeyFunc() to cache.DeletionHandlingMetaNamespaceKeyFunc()
	// to avoid importing k8s.io/kubernetes directly which is not supported.
	// key, err := controller.KeyFunc(curJob)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(curJob)
	if err != nil {
		return
	}
	jm.enqueueController(curJob, true)
	// check if need to add a new rsync for ActiveDeadlineSeconds
	if curJob.Status.StartTime != nil {
		curADS := curJob.Spec.ActiveDeadlineSeconds
		if curADS == nil {
			return
		}
		oldADS := oldJob.Spec.ActiveDeadlineSeconds
		if oldADS == nil || *oldADS != *curADS {
			now := metav1.Now()
			start := curJob.Status.StartTime.Time
			passed := now.Time.Sub(start)
			total := time.Duration(*curADS) * time.Second
			// AddAfter will handle total < passed
			jm.queue.AddAfter(key, total-passed)
			klog.V(4).Infof("job ActiveDeadlineSeconds updated, will rsync after %d seconds", total-passed)
		}
	}
}

// obj could be an *batch.Job, or a DeletionFinalStateUnknown marker item,
// immediate tells the controller to update the status right away, and should
// happen ONLY when there was a successful pod run.
func (jm *jobController) enqueueController(obj interface{}, immediate bool) {
	// lijj32: change controller.KeyFunc() to cache.DeletionHandlingMetaNamespaceKeyFunc()
	// to avoid importing k8s.io/kubernetes directly which is not supported.
	// key, err := controller.KeyFunc(obj)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}

	backoff := time.Duration(0)
	if !immediate {
		backoff = getBackoff(jm.queue, key)
	}

	// TODO: Handle overlapping controllers better. Either disallow them at admission time or
	// deterministically avoid syncing controllers that fight over pods. Currently, we only
	// ensure that the same controller is synced for a given pod. When we periodically relist
	// all controllers there will still be some replica instability. One way to handle this is
	// by querying the store for all controllers that this rc overlaps, as well as all
	// controllers that overlap this rc, and sorting them.
	jm.queue.AddAfter(key, backoff)
}

func getBackoff(queue workqueue.RateLimitingInterface, key interface{}) time.Duration {
	exp := queue.NumRequeues(key)

	if exp <= 0 {
		return time.Duration(0)
	}

	// The backoff is capped such that 'calculated' value never overflows.
	backoff := float64(DefaultJobBackOff.Nanoseconds()) * math.Pow(2, float64(exp-1))
	if backoff > math.MaxInt64 {
		return MaxJobBackOff
	}

	calculated := time.Duration(backoff)
	if calculated > MaxJobBackOff {
		return MaxJobBackOff
	}
	return calculated
}

// syncJob will sync the job with the given key if it has had its expectations fulfilled, meaning
// it did not expect to see any more of its pods created or deleted. This function is not meant to be invoked
// concurrently with the same key.
func (jm *jobController) syncJob(key string) (bool, error) {
	startTime := time.Now()
	defer func() {
		klog.V(5).Infof("Finished syncing job %q (%v)", key, time.Since(startTime))
	}()

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return false, err
	}
	if len(ns) == 0 || len(name) == 0 {
		return false, fmt.Errorf("invalid job key %q: either namespace or name is missing", key)
	}

	job, err := jm.jobLister.Jobs(ns).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Job has been deleted: %v", key)
			return true, nil
		}
		return false, err
	}

	err = jm.whenJobDone(job)
	if err != nil {
		jm.recorder.Event(job, corev1.EventTypeWarning, "AfterJobDone", err.Error())
	}

	return err == nil, err
}

func (jm *jobController) whenJobDone(job *batchv1.Job) error {
	ok, typ := isJobFinished(job)
	if !ok {
		return nil
	}

	kubeClient := jm.kubeClient

	var filesize int64 = 0
	logs, logerr := getJobLogs(kubeClient, job)
	if logerr != nil {
		logs = logerr.Error()
	}
	jobret := struct {
		Size int `json:"size"`
	}{}

	err := json.Unmarshal([]byte(logs), &jobret)
	if err == nil {
		filesize = int64(jobret.Size)
	}

	err = jm.detachJob(job)
	if err != nil {
		return err
	}

	bf, err := jm.mbf.GetFile(job.Name)
	if model.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if bf.Status != model.BackupFileRunning && !bf.FinishedAt.IsZero() {
		return nil
	}

	bf.Size = filesize
	bf.Status = string(typ)
	bf.FinishedAt = time.Now()
	bf.ExpiredAt = bf.FinishedAt.Add(bf.ExpiredAt.Sub(bf.CreatedAt))

	return jm.mbf.BackupJobDone(bf)
}

func isJobFinished(j *batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range j.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}

	return false, ""
}

func (jm *jobController) detachJob(job *batchv1.Job) error {

	pods, err := getPodsForJobV2(jm.kubeClient, job)
	if err != nil {
		return err
	}

	for _, pod := range pods {

		err := jm.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	errs := []error{}

	for _, pod := range pods {

		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName == "" {
				continue
			}

			pvc, err := jm.kubeClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.TODO(), v.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			err = jm.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				errs = append(errs, err)
			}

			if pvc.Spec.VolumeName == "" {
				continue
			}

			err = jm.kubeClient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvc.Spec.VolumeName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}
	}

	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName == "" {
			continue
		}

		pvc, err := jm.kubeClient.CoreV1().PersistentVolumeClaims(job.Namespace).Get(context.TODO(), v.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = jm.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}

		if pvc.Spec.VolumeName == "" {
			continue
		}

		err = jm.kubeClient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvc.Spec.VolumeName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}
