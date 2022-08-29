package bankend

import (
	"encoding/json"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Example_backupTemplate() {
	n := int32(1)
	seconds := int64(1)
	dir := corev1.HostPathDirectory

	resources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("1Gi"),
	}

	job := batchv1.Job{
		Spec: batchv1.JobSpec{
			Parallelism:           &n,
			Completions:           &n,
			ActiveDeadlineSeconds: &seconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName:      `{{printf "%q" .nodeName}}`,
					HostIPC:       true,
					HostNetwork:   true,
					RestartPolicy: corev1.RestartPolicyNever,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: `{{printf "%q" .imagePullSecret}}`,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: `{{printf "%q" .unit_name_data}}`,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: `{{printf "%q" .unit_name_data_path}}`,
									Type: &dir,
								},
							},
						},
						{
							Name: `{{printf "%q" .unit_name_log}}`,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: `{{printf "%q" .unit_name_log_path}}`,
									Type: &dir,
								},
							},
						},
						{
							Name: `{{printf "%q" .unit_name_backup_pvc}}`,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: `{{printf "%q" .unit_name_backup_pvc}}`,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  `{{printf "%q" .containerName}}`,
							Image: `{{printf "%q" .image}}`,
							Command: []string{
								"/root/backupMGR", `{{printf "%q" .backup_type}}`, "backup",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CREATE_TIME",
									Value: `{{printf "%q" .timestamp}}`,
								},
								{
									Name:  "BACKUP_ID",
									Value: `{{printf "%q" .backup_id}}`,
								},
								{
									Name:  "CPU_NUM",
									Value: `{{printf "%q" .CPU_NUM}}`,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: resources,
								Limits:   resources,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      `{{printf "%q" .unit_name_data}}`,
									ReadOnly:  true,
									MountPath: "/mysqldata",
								},
								{
									Name:      `{{printf "%q" .unit_name_log}}`,
									ReadOnly:  true,
									MountPath: "/mysqllog",
								},
								{
									Name:      `{{printf "%q" .unit_name_backup_pvc}}`,
									ReadOnly:  false,
									MountPath: "/mysqlbackup",
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(job)
	fmt.Println(string(data))

	_, err = backupJobTemplate(map[string]string{
		"unit_name_data":       "unit-name-data",
		"unit_name_data_path":  "unit_name_data_path",
		"unit_name_log":        "unit_name_log",
		"unit_name_log_path":   "unit_name_log_path",
		"unit_name_backup_pvc": "unit_name_backup_pvc",
		"containerName":        "containerName",
		"image":                "image",
		"backup_type":          "backup_type",
		"timestamp":            "timestamp",
		"backup_id":            "backup_id",
		"CPU_NUM":              "1",
		"nodeName":             "nodeName",
		"imagePullSecretName":  "imagePullSecretName",
	})
	if err != nil {
		fmt.Println(err)
	}
}
