package bankend

import (
	"encoding/json"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/pkg/structs"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	"path/filepath"
	"strings"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	//sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/zone/site"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type restoreJob struct {
	mu       model.Unit
	site     model.Site
	file     model.BackupFile
	endpoint model.BackupEndpoint
	app      model.Application

	zone zoneIface
	unit *unitv4.Unit
	pod  *corev1.Pod
	job  *batchv1.Job
	//lg   *sanv1.Lungroup
}

func (jr *restoreJob) Run(iface site.Interface, file, timestamp string) (bool, error) {

	unit, err := iface.Units().Get(jr.mu.Namespace, jr.mu.ObjectName())
	if err != nil {
		return false, err
	}

	pod, err := iface.Pods().Get(unit.Namespace, unit.PodName())
	if err != nil {
		return false, err
	}

	if unit.Spec.Action.Delete != nil || unit.Spec.Action.Rebuild != nil {
		return true, nil
	}

	jr.unit = unit
	jr.pod = pod

	// first time execution
	if jr.job == nil {
		// keep pod running but not ready
		// need to wait some times before return
		retries := 0
		for {
			if !podutil.IsRunning(pod) || podutil.IsRunningAndReady(pod) {
				retries += 1
				if retries > 12 {
					return false, fmt.Errorf("pod %s/%s keep running but not ready,current phase is %s", pod.Namespace, pod.Name, pod.Status.Phase)
				} else {
					time.Sleep(time.Second * 10)
					pod, err = iface.Pods().Get(unit.Namespace, unit.PodName())
					if err != nil {
						return false, err
					}
				}
			} else {
				break
			}
		}

		err := jr.createRestoreJob(iface, file, timestamp)
		if err != nil {
			return false, err
		}

		return false, nil
	} else {
		job, err := iface.Jobs().Get(jr.job.Namespace, jr.job.Name)
		if err != nil {
			return false, err
		}

		jr.job = job
		ok, typ := isJobFinished(jr.job)
		if !ok {
			return false, nil
		}
		if typ == batchv1.JobComplete {
			_, err = jr.zone.updateUnitAction(jr.site.ID, jr.unit.Namespace, jr.unit.Name, api.StatePassing)
			if err != nil {
				return false, err
			}

			retries := 0
			for {
				retries += 1
				pod, err = iface.Pods().Get(unit.Namespace, unit.PodName())
				if retries > 12 {
					return false, stderror.New("pod not ready in 2 minutes")
				}
				if !podutil.IsRunningAndReady(pod) {
					time.Sleep(time.Second * 10)
				} else { //if pod is ready to use
					return ok, nil
				}
			}

		} else {
			return false, stderror.New("job not finished")
		}
	}
}

func (jr *restoreJob) createRestoreJob(iface site.Interface, file, timestamp string) error {
	values, err := jr.jobParams(iface, file, timestamp)
	if err != nil {
		return err
	}

	tmpl, err := restoreJobTemplate(values)
	if err != nil {
		return err
	}

	job, err := iface.Jobs().Get(tmpl.Namespace, tmpl.Name)
	if errors.IsNotFound(err) {
		job, err = iface.Jobs().Create(tmpl.Namespace, &tmpl)
	}
	if err != nil {
		return err
	}

	err = jr.createRelatedRestorePVC(iface, values)
	if err != nil {
		return err
	}

	jr.job = job

	return err
}

func (jr restoreJob) jobParams(iface site.Interface, file, timestamp string) (map[string]string, error) {
	var err error
	values := make(map[string]string)
	namespace := jr.unit.GetNamespace()

	values["image"], values["toolkit_script"], err = backupImage(jr.app, jr.site)
	if err != nil {
		return values, err
	}
	//values["backupfile"] = jr.file
	values["unit_name"] = jr.unit.Name
	values["jobName"] = fmt.Sprintf("%s-%s-full-restore", values["unit_name"], timestamp)
	values["fileName"] = file
	values["service_name"] = jr.app.ID
	values["service_config"] = fmt.Sprintf("%s-service-config", jr.unit.Name)
	values["namespace"] = namespace
	values["nodeName"] = jr.pod.Spec.NodeName
	values["containerName"] = "restore"
	values[string(corev1.ResourceCPU)] = "1"
	values[string(corev1.ResourceMemory)] = "2048Mi"

	values["storage_type"] = jr.endpoint.Type
	values["backup_type"] = jr.file.Type
	//values["imagePullSecret"] = bfoptions["imagePullSecret"]
	values["hostpath_volume"] = structs.NFSBackupTarget
	//get the createtime from file name ...
	// rrrr-mysql0000-cce9c72d-1-full-1595380760 -> 1595380760
	fileNameGroups := strings.Split(jr.file.File, "-")
	values["src_timestamp"] = fileNameGroups[len(fileNameGroups)-2]
	values["src_service_name"] = jr.file.App
	values["src_unit_name"] = jr.file.Unit
	values["nfs-pvc-name"] = values["jobName"] + "-" + jr.endpoint.Type

	for _, secret := range jr.pod.Spec.ImagePullSecrets {
		values["imagePullSecret"] = secret.Name
	}

	if jr.endpoint.Type == structs.S3BackupStorageType {
		var s3Config api.BackupEndpointS3Config
		err = json.Unmarshal([]byte(jr.endpoint.Config), &s3Config)
		if err != nil {
			return nil, err
		}

		values["s3_url"] = s3Config.S3Url
		values["s3_access_key"] = s3Config.S3AcKey
		values["s3_secret_key"] = s3Config.S3Secret
		values["s3_hostbucket"] = s3Config.S3Bucket
	}

	for _, claim := range jr.unit.Spec.VolumeClaims {
		if claim.Name == "log" {
			values["unit_name_log"] = claim.Name
			values["unit_name_log_path"] = getCSImountPath(jr.pod, unitv4.GetVolumePathName(jr.unit, claim.Name))

		} else if claim.Name == "data" {
			values["unit_name_data"] = claim.Name
			values["unit_name_data_path"] = getCSImountPath(jr.pod, unitv4.GetVolumePathName(jr.unit, claim.Name))
		}
	}

	template, err := iface.ConfigMaps().Get(jr.unit.Namespace, unitv4.GetTemplateConfigName(jr.unit))
	if err != nil {
		return nil, err
	}
	cnfPath, ok := template.Data[unitv4.ConfigFilePathTab]
	if !ok {
		return nil, err
	}

	values["config_path"] = cnfPath

	return values, nil
}

func restoreJobTemplate(values map[string]string) (batchv1.Job, error) {
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
	//resources := corev1.ResourceList{
	//	corev1.ResourceCPU:    cpu,
	//	corev1.ResourceMemory: memory,
	//}
	ttlSecond := int32(5 * 60)

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
		fmt.Errorf("%s:bacuptype not support now", storageType)
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
	scriptmounter := corev1.VolumeMount{Name: "script", MountPath: "/opt/app-root/scripts"}

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

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: values["namespace"],
			Name:      values["jobName"],
		},

		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSecond,
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
								structs.EntranceScript, "backupfile", "restore",
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
									Value: values["src_timestamp"],
								},
								{
									Name:  "SERVICE",
									Value: values["src_service_name"],
								},
								{
									Name:  "UNIT",
									Value: values["src_unit_name"],
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
								scriptmounter,
								customCnfmounter,
								{
									Name:      values["unit_name_data"],
									ReadOnly:  false,
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

func (jr restoreJob) createRelatedRestorePVC(iface site.Interface, values map[string]string) error {

	backupEndpointId := jr.endpoint.ID
	backupEndpoint := jr.endpoint
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
	claim.ObjectMeta.Annotations["nfs.io/directory-name"] = values["fileName"] + "-" + values["storage_type"]

	_, err := iface.PersistentVolumeClaims().Create(corev1.NamespaceDefault, claim)
	return err
}
