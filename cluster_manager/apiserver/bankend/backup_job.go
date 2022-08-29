package bankend

import (
	"encoding/json"
	"fmt"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/structs"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"github.com/upmio/dbscale-kube/pkg/zone/site"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"

	//"strings"
	"time"
)

var (
	removeExpiredBackupFiles = true
	gracePeriodSeconds       = int64(300)
	propagationPolicy        = metav1.DeletePropagationForeground
	jobKind                  = batchv1.SchemeGroupVersion.WithKind("Job")
)

type backupStrategy struct {
	apps   appGetter
	sites  siteGetter
	getter strategyGetter
	mbf    modelBackupFile
	mbep   modelBackupEndpoint
	zone   zoneIface

	schedule string
	strategy model.BackupStrategy
	app      model.Application
}

func (bs backupStrategy) Run() {

	go bs.deleteExpiredBackupFiles()

	ok, err := bs.preRun()
	if err != nil {
		klog.Error(err)
		return
	}

	if !ok {
		return
	}

	var (
		last      *jobRelates
		timestamp = strconv.Itoa(int(time.Now().Unix()))
	)

	wt := NewWaitTask(time.Second*30, nil)
	err = wt.WithTimeout(time.Minute*3, func() (bool, error) {

		jr, err := bs.run(timestamp, last)
		last = jr

		return err == nil, err
	})

	if err != nil {
		klog.Errorf("backup strategy %s,error:%s", bs.strategy.ID, err)
	}
}

func getCSImountPath(pod *corev1.Pod, pvname string) string {
	return fmt.Sprintf("/var/lib/kubelet/pods/%s/volumes/kubernetes.io~csi/%s/mount", pod.GetUID(), pvname)
}

func (bs backupStrategy) run(timestamp string, last *jobRelates) (*jobRelates, error) {
	jobUnits, err := bs.backupJobUnits()

	if len(jobUnits) == 0 {
		return nil, fmt.Errorf("app %s without running pod,error:%v", bs.app.ID, err)
	}
	if err != nil {
		klog.Warning(err)
	}

	index := 0
	exist := false

	if bs.strategy.Unit != "" {

		for i := range jobUnits {
			if jobUnits[i].mu.ID == bs.strategy.Unit {
				index = i
				exist = true
				break
			}
		}

	} else {

		sortByRole(jobUnits)

		if bs.strategy.Role == "" {
			index = 0
			exist = true

		} else {

			for i := range jobUnits {
				if jobUnits[i].repl.Role == bs.strategy.Role {
					index = i
					exist = true
					break
				}
			}
		}
	}

	if !exist {
		return nil, fmt.Errorf("not found unit matches unit '%s' or role '%s'", bs.strategy.Unit, bs.strategy.Role)
	}

	backupJob := jobUnits[index]
	backupJob.strategy = bs.strategy

	if !backupJob.running {
		return last, fmt.Errorf("pod %s current phase is %s,try backup later", backupJob.pod.Name, backupJob.pod.Status.Phase)
	}

	if last != nil && last.unit.ID != backupJob.mu.ID {
		err := bs.waitForDeleteLastJob(last)
		if err != nil {
			return last, err
		}
	}

	iface, err := bs.zone.siteInterface(bs.zone.GetSite())
	if err != nil {
		return nil, err
	}

	site, err := bs.sites.Get(bs.zone.GetSite())
	if err != nil {
		return nil, err
	}

	values := make(map[string]string)
	namespace := backupJob.unit.GetNamespace()
	values["image"], values["toolkit_script"], err = backupImage(bs.app, site)
	if err != nil {
		return nil, err
	}

	values["service_config"] = fmt.Sprintf("%s-service-config", backupJob.unit.Name)
	values["service_name"] = bs.strategy.App
	values["namespace"] = namespace
	values["unit_name"] = backupJob.unit.Name
	values["nodeName"] = backupJob.pod.Spec.NodeName
	values["containerName"] = "backup"
	values[string(corev1.ResourceCPU)] = "1"
	values[string(corev1.ResourceMemory)] = "2048Mi"
	values["backup_type"] = bs.strategy.Type
	values["timestamp"] = timestamp
	values["jobName"] = fmt.Sprintf("%s-%s-%s", values["unit_name"], values["timestamp"], values["backup_type"])
	values["backupFileName"] = fmt.Sprintf("%s-%s-%s", values["unit_name"], values["timestamp"], values["backup_type"])
	for _, secret := range backupJob.pod.Spec.ImagePullSecrets {
		values["imagePullSecret"] = secret.Name
	}

	backupEndpointId := bs.strategy.EndpointId
	backupEndpoint, err := bs.mbep.GetEndpoint(backupEndpointId)
	if err != nil {
		return nil, err
	}
	values["storage_type"] = backupEndpoint.Type
	switch backupEndpoint.Type {
	case structs.NFSBackupStorageType:
		values["nfs-pvc-name"] = values["jobName"] + "-" + backupEndpoint.Type
		values["nfs-provider"] = backupEndpointId
	case structs.S3BackupStorageType:
		var s3Config api.BackupEndpointS3Config
		err = json.Unmarshal([]byte(backupEndpoint.Config), &s3Config)
		if err != nil {
			return nil, err
		}

		values["s3_url"] = s3Config.S3Url
		values["s3_access_key"] = s3Config.S3AcKey
		values["s3_secret_key"] = s3Config.S3Secret
		values["s3_hostbucket"] = s3Config.S3Bucket

	default:
		return nil, fmt.Errorf(":backup type %s not support now", backupEndpoint.Type)
	}

	for _, claim := range backupJob.unit.Spec.VolumeClaims {
		if claim.Name == "log" {
			values["unit_name_log"] = claim.Name
			values["unit_name_log_path"] = getCSImountPath(backupJob.pod, unitv4.GetVolumePathName(backupJob.unit, claim.Name))

		} else if claim.Name == "data" {
			values["unit_name_data"] = claim.Name
			values["unit_name_data_path"] = getCSImountPath(backupJob.pod, unitv4.GetVolumePathName(backupJob.unit, claim.Name))

		}
	}

	template, err := iface.ConfigMaps().Get(backupJob.unit.Namespace, unitv4.GetTemplateConfigName(backupJob.unit))
	if err != nil {
		return nil, err
	}
	cnfPath, ok := template.Data[unitv4.ConfigFilePathTab]
	if !ok {
		return nil, err
	}

	values["config_path"] = cnfPath
	job, err := bs.createBackupJob(iface, values)
	if err != nil {
		return nil, err
	}
	err = bs.createRelatedBackupPVC(iface, values)
	if err != nil {
		return nil, err
	}

	jobRelate := &jobRelates{job: job}
	err = bs.createBackupFiles(backupJob, jobRelate, values["backupFileName"])
	if err != nil {
		return jobRelate, err
	}
	// register jobController,handle BackupFile after job done
	jm := jobController{mbf: bs.mbf}
	iface.RegisterController("jobControllerKey", jm.Run)

	return jobRelate, err
}

func (bs *backupStrategy) preRun() (bool, error) {

	// wait 10s max
	time.Sleep(time.Duration(rand.Intn(10000)) * time.Millisecond)

	s, err := bs.getter.Lock(bs.strategy.ID)
	if model.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	bs.strategy = s

	// (bs.schedule != s.Schedule)==true means the strategy has updated,so skip run
	if !s.Enabled || bs.schedule != s.Schedule {
		return false, nil
	}

	app, err := bs.apps.Get(s.App)
	if model.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	bs.app = app

	return true, nil
}

func backupJobTemplate(values map[string]string) (batchv1.Job, error) {
	n := int32(1)
	dir := corev1.HostPathDirectory
	ncpu := values[string(corev1.ResourceCPU)]

	//cpu, err := resource.ParseQuantity(ncpu)
	//if err != nil {
	//	return batchv1.Job{}, err
	//}
	//memory, err := resource.ParseQuantity(values[string(corev1.ResourceMemory)])
	//if err != nil {
	//	return batchv1.Job{}, err
	//}
	//
	//resources := corev1.ResourceList{
	//	corev1.ResourceCPU:    cpu,
	//	corev1.ResourceMemory: memory,
	//}
	storageType := values["storage_type"]
	backupVolume := corev1.Volume{Name: storageType}
	switch storageType {
	case structs.NFSBackupStorageType:
		backupVolume.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: values["nfs-pvc-name"],
		}
	case structs.S3BackupStorageType:
		klog.Info("s3 is used")
	default:
		fmt.Errorf(":backup type %s not support now", storageType)
	}

	mode := int32(0755)
	scriptVolume := corev1.Volume{Name: "script"}
	scriptVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: values["toolkit_script"],
		},
		Items: []corev1.KeyToPath{
			corev1.KeyToPath{
				Key:  "scripts",
				Path: "unitMGR",
				Mode: &mode,
			},
		},
	}
	scriptMounter := corev1.VolumeMount{Name: "script", MountPath: "/opt/app-root/scripts"}

	customCnfmode := int32(0644)
	customCnfvolume := corev1.Volume{Name: "custom-config"}
	customCnfvolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: values["service_config"],
		},
		Items: []corev1.KeyToPath{
			corev1.KeyToPath{
				Key:  unitv4.ConfigDataTab,
				Path: filepath.Base(values["config_path"]),
				Mode: &customCnfmode,
			},
		},
	}
	customCnfmounter := corev1.VolumeMount{Name: "custom-config", MountPath: "/opt/app-root/configs"}

	ttlsecond := int32(5 * 60)
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: values["namespace"],
			Name:      values["jobName"] + "-" + values["containerName"],
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlsecond,
			BackoffLimit:            &n,
			Parallelism:             &n,
			Completions:             &n,
			// ActiveDeadlineSeconds: &seconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName:      values["nodeName"],
					HostIPC:       true,
					HostNetwork:   true,
					RestartPolicy: corev1.RestartPolicyNever,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: values["imagePullSecret"],
						},
					},
					Containers: []corev1.Container{
						{
							Name:  values["containerName"],
							Image: values["image"],
							Command: []string{
								structs.EntranceScript, "backupfile", "create",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DATA_MOUNT",
									Value: structs.DefaultDataMount,
								},
								{
									Name:  "LOG_MOUNT",
									Value: structs.DefaultLogMount,
								},
								{
									Name:  "BACKUP_TYPE",
									Value: values["backup_type"],
								},
								{
									Name:  "CREATE_TIME",
									Value: values["timestamp"],
								},
								{
									Name:  "SERVICE",
									Value: values["service_name"],
								},
								{
									Name:  "UNIT",
									Value: values["unit_name"],
								},
								{
									Name:  "STORAGE_TYPE",
									Value: storageType,
								},
								{
									Name:  "CPU_NUM",
									Value: ncpu,
								},
								{
									Name:  "CONFIG_PATH",
									Value: values["config_path"],
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								scriptMounter,
								customCnfmounter,
								{
									Name:      values["unit_name_data"],
									ReadOnly:  true,
									MountPath: structs.DefaultDataMount,
								},
								{
									Name:      values["unit_name_log"],
									ReadOnly:  false,
									MountPath: structs.DefaultLogMount,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						scriptVolume,
						customCnfvolume,
						backupVolume,
						{
							Name: values["unit_name_data"],
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: values["unit_name_data_path"],
									Type: &dir,
								},
							},
						},
						{
							Name: values["unit_name_log"],
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: values["unit_name_log_path"],
									Type: &dir,
								},
							},
						},
					},
				},
			},
		},
	}

	if _, ok := values["s3_url"]; ok {
		var s3EnvVars = []corev1.EnvVar{
			{
				Name:  "S3_URL",
				Value: values["s3_url"],
			},
			{
				Name:  "ACCESS_KEY",
				Value: values["s3_access_key"],
			},
			{
				Name:  "SECRET_KEY",
				Value: values["s3_secret_key"],
			},
			{
				Name:  "HOSTBUCKET",
				Value: values["s3_hostbucket"],
			},
		}

		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, s3EnvVars...)
	} else {

		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "BACKUP_MOUNT",
			Value: structs.DefaultBACKMount,
		})

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      values["storage_type"],
			ReadOnly:  false,
			MountPath: structs.DefaultBACKMount,
		})
	}

	return job, nil
}

type backupJob struct {
	running  bool
	size     resource.Quantity
	strategy model.BackupStrategy
	unit     *unitv4.Unit
	pod      *corev1.Pod

	mu   model.Unit
	repl api.Replication
}

type jobRelates struct {
	job      *batchv1.Job
	pv       *corev1.PersistentVolume
	pvc      *corev1.PersistentVolumeClaim
	lungroup *sanv1.Lungroup

	unit model.Unit
	bf   model.BackupFile
}

func sortByRole(jobs []backupJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if !jobs[i].running {
			return false
		}
		if jobs[i].running && !jobs[j].running {
			return true
		}

		if jobs[i].repl.Role == "slave" && jobs[j].repl.Role != "slave" {
			return true
		}

		if jobs[i].repl.ReplicationSlaveInfo != nil &&
			jobs[j].repl.ReplicationSlaveInfo == nil {
			return true
		}

		if jobs[i].repl.ReplicationSlaveInfo != nil &&
			jobs[j].repl.ReplicationSlaveInfo != nil {

			return jobs[i].repl.SecondsBehindMaster < jobs[j].repl.SecondsBehindMaster ||
				jobs[i].repl.MasterLogPos > jobs[j].repl.MasterLogPos
		}

		return false
	})
}

func (bs *backupStrategy) backupJobUnits() ([]backupJob, error) {
	var errs []error
	jobs := make([]backupJob, 0, len(bs.app.Units))

	for _, u := range bs.app.Units {

		iface, err := bs.zone.siteInterface(u.Site)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		unit, err := iface.Units().Get(u.Namespace, u.ObjectName())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		pod, err := iface.Pods().Get(unit.Namespace, unit.PodName())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		repl, err := getUnitReplication(iface.PodExec(), *unit)
		if err != nil {
			errs = append(errs, err)
		}

		jobs = append(jobs, backupJob{
			running: podutil.IsRunning(pod),
			mu:      u,
			unit:    unit,
			pod:     pod,
			repl:    repl,
		})
	}

	return jobs, utilerrors.NewAggregate(errs)
}

func (bs *backupStrategy) createBackupFiles(job backupJob, jr *jobRelates, file string) error {

	now := time.Now()
	bf := model.BackupFile{
		Size:        job.size.Value() >> 20,
		Status:      model.BackupFileRunning,
		ID:          "",
		File:        file,
		Type:        job.strategy.Type,
		EndpointId:  job.strategy.EndpointId,
		Site:        bs.zone.GetSite(),
		Namespace:   job.unit.Namespace,
		App:         job.mu.App,
		Unit:        job.mu.ID,
		Job:         jr.job.Name,
		Strategy:    job.strategy.ID,
		CreatedUser: job.strategy.CreatedUser,
		ExpiredAt:   now.AddDate(0, 0, job.strategy.Retention),
		CreatedAt:   now,
		FinishedAt:  time.Time{},
	}

	id, err := bs.mbf.InsertFile(bf)
	if err == nil {
		bf.ID = id
		jr.bf = bf
	}

	return err
}

func (bs *backupStrategy) createBackupJob(iface site.Interface, values map[string]string) (*batchv1.Job, error) {

	obj, err := backupJobTemplate(values)
	if err != nil {
		return nil, err
	}

	job, err := iface.Jobs().Get(obj.Namespace, obj.Name)
	if errors.IsNotFound(err) {
		job, err = iface.Jobs().Create(obj.Namespace, &obj)
	}

	return job, err
}

func (bs *backupStrategy) createRelatedBackupPVC(iface site.Interface, values map[string]string) error {

	backupEndpointId := bs.strategy.EndpointId
	backupEndpoint, err := bs.mbep.GetEndpoint(backupEndpointId)
	if err != nil {
		return err
	}
	if backupEndpoint.Type != structs.NFSBackupStorageType {
		return nil
	}

	provisionerName := fmt.Sprintf("nfs-provisioner-%s", backupEndpointId[:8])

	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   corev1.NamespaceDefault,
			Name:        values["jobName"] + "-" + values["storage_type"],
			Annotations: make(map[string]string),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &provisionerName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: *resource.NewQuantity(10*1024*1024, resource.BinarySI),
				},
			},
		},
	}
	claim.ObjectMeta.Annotations["nfs.io/directory-name"] = values["jobName"] + "-" + values["storage_type"]

	_, err = iface.PersistentVolumeClaims().Create(corev1.NamespaceDefault, claim)
	return err
}

func (bs *backupStrategy) volumeCapacity(execer site.PodExecInterface, job backupJob) (resource.Quantity, error) {
	q := resource.Quantity{}

	volumes, err := getUnitVolumesUsage(execer, *job.unit)
	if err != nil {
		return q, err
	}

	// ( {{data.used}} + {{log.used}} ) * 1.2 = {{backup.capacity}}
	used := 0
	for _, v := range volumes {
		if v.Type == "data" || v.Type == "log" {
			used += v.Used
		}
	}

	return convertMiToQuantity(int64(used) * 12 / 10)
}

func backupImage(app model.Application, site model.Site) (string, string, error) {
	// toolkit:mysql-5.7.24.1-amd64
	spec, err := decodeAppSpec(app.Spec)
	if err != nil {
		return "", "", err
	}

	im := model.Image{
		ImageVersion: model.ImageVersion(spec.Database.Image),
	}
	toolKitCm := fmt.Sprintf("%s-toolkit-script", im.ImageTemplateFileNameWithArch())

	return fmt.Sprintf("%s/%s/%s", site.ImageRegistry, site.ProjectName, "toolkit:"+im.ImageTemplateFileNameWithArch()), toolKitCm, nil
}

func (bs *backupStrategy) waitForDeleteLastJob(jr *jobRelates) error {
	if jr.job == nil && jr.lungroup == nil && jr.pv == nil && jr.pvc == nil {
		return nil
	}

	wt := NewWaitTask(time.Second*10, nil)
	return wt.WithTimeout(time.Minute, func() (bool, error) {

		iface, err := bs.zone.siteInterface(jr.unit.Site)
		if err != nil {
			return false, err
		}

		pods, err := getPodsForJobFromSiteInterface(iface, jr.job)
		if err != nil {
			return false, err
		}

		for _, pod := range pods {
			err := iface.Pods().Delete(pod.Namespace, pod.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}

		if jr.pvc != nil {
			err := iface.PersistentVolumeClaims().Delete(jr.pvc.Namespace, jr.pvc.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
			jr.pvc = nil
		}

		if jr.pv != nil {
			err := iface.PersistentVolumes().Delete(jr.pv.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
			jr.pv = nil
		}

		if jr.job != nil {
			err := iface.Jobs().Delete(jr.job.Namespace, jr.job.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
				PropagationPolicy:  &propagationPolicy, // grace delete include pods
			})
			if err == nil || errors.IsNotFound(err) {
				jr.job = nil
				return true, nil

			} else {
				return false, err
			}
		}

		return false, nil
	})
}

func (bs backupStrategy) deleteExpiredBackupFiles() error {
	if !removeExpiredBackupFiles {
		return nil
	}

	err := deleteExpiredBackupFiles(bs.mbf, bs.zone)
	if err != nil {
		klog.Errorf("delete expired backup files:%s", err)
	}

	return err
}
