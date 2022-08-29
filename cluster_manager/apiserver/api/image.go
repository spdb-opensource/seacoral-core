package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type Image struct {
	ImageVersion
	Unschedulable bool      `json:"unschedulable"`
	Site          IDName    `json:"site"`
	ExporterPort  int       `json:"exporter_port"`
	Desc          string    `json:"desc"`
	Task          TaskBrief `json:"task"`
	Created       Editor    `json:"created"`
	Modified      Editor    `json:"modified"`
}

type ImageConfig struct {
	// 镜像版本
	ImageVersion
	Site          string `json:"site_id"`
	ExporterPort  int    `json:"exporter_port"`
	Unschedulable bool   `json:"unschedulable"`
	Desc          string `json:"desc"`
	User          string `json:"created_user"`
}

// 镜像版本
type ImageVersion struct {
	ID string `json:"id"`

	// Registry string `json:"-"`
	// 名称
	// example: mysql
	Type string `json:"type"`
	// Arch
	Arch string `json:"arch"`
	// 主版本号
	Major int `json:"major"`
	// 小版本号
	Minor int `json:"minor"`
	// 小更新版本号
	Patch int `json:"patch"`
	// 编译版本号
	Dev int `json:"build"`
}

func (iv ImageVersion) String() string {
	return fmt.Sprintf("%s:%d.%d.%d.%d", iv.Type, iv.Major, iv.Minor, iv.Patch, iv.Dev)
}

func ParseImageVersion(image string) (ImageVersion, error) {
	ierr := errors.New("parse image:" + image)
	iv := ImageVersion{}
	ss := strings.Split(image, "/")

	// registry1.service.consul:20160/project_name/mysql:5.7.25.12-amd64
	// mysql:5.7.25.12-amd64
	parts := strings.Split(ss[len(ss)-1], ":")
	iv.Type = parts[0]

	if len(parts) == 2 {
		//mysql:5.7.25.12-amd64 -> mysql:5.7.25.12.amd64
		v := strings.Split(strings.Replace(parts[1], "-", ".", 1), ".")

		if n, err := strconv.Atoi(v[0]); err == nil {
			iv.Major = n
		} else {
			return iv, ierr
		}

		if len(v) > 1 {
			if n, err := strconv.Atoi(v[1]); err == nil {
				iv.Minor = n
			} else {
				return iv, ierr
			}
		}
		if len(v) > 2 {

			if n, err := strconv.Atoi(v[2]); err == nil {
				iv.Patch = n
			} else {
				return iv, ierr
			}
		}
		if len(v) > 3 {
			if n, err := strconv.Atoi(v[3]); err == nil {
				iv.Dev = n
			} else {
				return iv, ierr
			}
		}
		if len(v) > 4 {
			iv.Arch = v[4]
		}
	}

	iv.ID = iv.String()

	return iv, nil
}

func (c ImageConfig) Valid() error {
	var errs []error

	if c.Type == "" {
		errs = append(errs, xerrors.New("image type is required"))
	}

	if c.Site == "" {
		errs = append(errs, xerrors.New("site is required"))
	}

	if c.Major < 0 {
		errs = append(errs, xerrors.New("image major version is required"))
	}

	if c.Minor < 0 {
		errs = append(errs, xerrors.New("image minor version is required"))
	}

	if c.Patch < 0 {
		errs = append(errs, xerrors.New("image patch version is required"))
	}

	if c.Dev < 0 {
		errs = append(errs, xerrors.New("image build version is required"))
	}

	return utilerrors.NewAggregate(errs)
}

type ImagesResponse []Image

type ImageOptions struct {
	Unschedulable *bool   `json:"unschedulable,omitempty"`
	Desc          *string `json:"desc,omitempty"`
}

type ImageListOptions struct {
	Unschedulable *string `json:"unschedulable,omitempty"`

	ID     *string `json:"id,omitempty"`
	SiteId *string `json:"site_id,omitempty"`

	Type *string `json:"type,omitempty"`
	// 主版本号
	Major *string `json:"major,omitempty"`
	// 小版本号
	Minor *string `json:"minor,omitempty"`
	// 小更新版本号
	Patch *string `json:"patch,omitempty"`
	// 编译版本号
	Dev *string `json:"build,omitempty"`
}

//type Keyset struct {
//	CanSet      bool   `json:"can_set"`
//	MustRestart bool   `json:"must_restart"`
//	Key         string `json:"key"`
//	Value       string `json:"value"`
//	Default     string `json:"default"`
//	Desc        string `json:"desc"`
//	Range       string `json:"range"`
//}

type ConfigTemplate struct {
	ConfigFile string `json:"config_file"`

	Keysets []KeySet `json:"keysets"`

	//	Content    string   `json:"content,omitempty"`
}

type ImageTemplate struct {
	Image    ImageVersion   `json:"image"`
	Template ConfigTemplate `json:"template"`
}

type ImageScripts struct {
	Image         ImageVersion `json:"image"`
	ScriptContent string       `json:"script_content"`
}

type ImageTemplatesResponse []ImageTemplate

type ConfigTemplateOptions struct {
	Keysets []KeySet `json:"keysets"`
	//	Content　*string`json:"content"`
}

//
type KeySet struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Range       string `json:"range"`
	Default     string `json:"default"`
	CanSet      bool   `json:"can_set"`
	MustRestart bool   `json:"must_restart"`
	Desc        string `json:"desc"`
}

type Command struct {
	Command []string `json:"command"`
}

//type ImageTempate struct {
//	KeySets     []KeySet
//	PodTemplate PodTemplate
//	//ConfigTempalte string
//}

type PodTemplate struct {
	ReadinessProbe corev1.Probe `json:"readiness_probe"`
	LivenessProbe  corev1.Probe `json:"liveness_probe"`
}

func (p PodTemplate) Valid() error {
	errs := []error{}

	if p.ReadinessProbe.Exec == nil || len(p.ReadinessProbe.Exec.Command) == 0 {
		errs = append(errs, xerrors.New("readinessprobe.exec.command is required"))
	}

	if p.ReadinessProbe.FailureThreshold <= 0 {
		errs = append(errs, xerrors.New("failureThreshold cannot be 0"))
	}

	if p.ReadinessProbe.InitialDelaySeconds <= 0 {
		errs = append(errs, xerrors.New("initialDelaySeconds cannot be 0"))
	}

	if p.ReadinessProbe.PeriodSeconds <= 0 {
		errs = append(errs, xerrors.New("periodSeconds cannot be 0"))
	}

	if p.ReadinessProbe.TimeoutSeconds <= 0 {
		errs = append(errs, xerrors.New("timeoutSeconds cannot be 0"))
	}

	if p.LivenessProbe.Exec == nil || len(p.LivenessProbe.Exec.Command) == 0 {
		errs = append(errs, xerrors.New("LivenessProbe.exec.command is required"))
	}

	if p.LivenessProbe.FailureThreshold <= 0 {
		errs = append(errs, xerrors.New("LivenessProbe failureThreshold cannot be 0"))
	}

	if p.LivenessProbe.InitialDelaySeconds <= 0 {
		errs = append(errs, xerrors.New("LivenessProbe initialDelaySeconds cannot be 0"))
	}

	if p.LivenessProbe.PeriodSeconds <= 0 {
		errs = append(errs, xerrors.New("LivenessProbe periodSeconds cannot be 0"))
	}

	if p.LivenessProbe.TimeoutSeconds <= 0 {
		errs = append(errs, xerrors.New("LivenessProbe timeoutSeconds cannot be 0"))
	}

	return utilerrors.NewAggregate(errs)
}

type ImageTempate struct {
	KeySets        string `json:"key_sets"`
	PodTemplate    string `json:"pod_template"`
	ConfigTemplate string `json:"config_template"`
}
