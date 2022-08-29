package api

import (
	stderror "github.com/pkg/errors"
	cron "github.com/robfig/cron/v3"
	"github.com/xeipuuv/gojsonschema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"strings"
	"sync"
)

var backupJsonSchema []byte
var backupJsonSchemaLoaded bool
var backupJsonSchemaOnce sync.Once
var backupEpJsonSchema []byte
var backupEpJsonSchemaLoaded bool
var backupEpJsonSchemaOnce sync.Once

type BackupType string // 增备　全备

type BackupFilesResponse []BackupFile

type BackupFile struct {
	Valid bool  `json:"valid"`
	Size  int64 `json:"size"`

	ID   string `json:"id"`
	Name string `json:"name"`

	Unit         IDName     `json:"unit"`
	App          IDName     `json:"app"`
	Site         IDName     `json:"site"`
	Endpoint     IDName     `json:"endpoint"`
	EndpointType string     `json:"endpoint_type"`
	Path         string     `json:"path"`
	Type         BackupType `json:"type"`
	Status       string     `json:"status"`

	ExpiredAt  Time `json:"expire_at"`
	CreatedAt  Time `json:"create_at"`
	FinishedAt Time `json:"finish_at"`

	User string `json:"created_user"`
}

type BackupStrategy struct {
	Enabled   bool       `json:"enabled"`
	Retention int        `json:"retention"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Storage   string     `json:"storage"`
	App       string     `json:"app_id"`
	Unit      string     `json:"unit_id"`
	Schedule  string     `json:"schedule"`
	Desc      string     `json:"desc"`
	Role      string     `json:"role"`
	Type      BackupType `json:"type"`
	Tables    []string   `json:"tables"`

	Created  Editor `json:"created"`
	Modified Editor `json:"modified"`
}

type BackupStrategyResponse []BackupStrategy

type BackupStrategyConfig struct {
	Enabled      bool       `json:"enabled"`
	Once         bool       `json:"once"`
	Retention    int        `json:"retention"`
	Name         string     `json:"name"`
	EndpointId   string     `json:"endpoint_id,omitempty"`
	EndpointType string     `json:"endpoint_type,omitempty"`
	EndpointName string     `json:"endpoint_name,omitempty"`
	App          string     `json:"app_id"`
	Unit         string     `json:"unit_id,omitempty"`
	Schedule     string     `json:"schedule"`
	Desc         string     `json:"desc"`
	Role         string     `json:"role,omitempty"`
	BackupType   BackupType `json:"type"`
	Tables       []string   `json:"tables,omitempty"`
	User         string     `json:"created_user"`
}

func (config BackupStrategyConfig) Valid() error {
	backupJsonSchemaOnce.Do(func() {
		var err error
		backupJsonSchema, err = GetJsonSchema("backupStrategy.json")
		backupJsonSchemaLoaded = err == nil
	})

	if !backupJsonSchemaLoaded {
		return stderror.New("Failed to load json schema")
	}

	schemaLoader := gojsonschema.NewBytesLoader(backupJsonSchema)
	loader := gojsonschema.NewGoLoader(config)
	var errs []error

	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			errs = append(errs, stderror.Errorf("- %s\n", err))
		}
	}

	if strings.TrimSpace(config.Schedule) != "" {
		_, err = cron.ParseStandard(config.Schedule)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if !config.Once && strings.TrimSpace(config.Schedule) == "" {
		errs = append(errs, stderror.New("once is false but schedule is not provided"))
	}

	if config.Role == "" && config.Unit == "" {
		errs = append(errs, stderror.New("role and unit cannot both be null"))
	}

	return utilerrors.NewAggregate(errs)
}

type BackupStrategyOptions struct {
	Enabled    *bool   `json:"enabled,omitempty"`
	Retention  *int    `json:"retention,omitempty"`
	Schedule   *string `json:"schedule,omitempty"`
	App        *string `json:"app_id,omitempty"`
	Unit       *string `json:"unit_id,omitempty"`
	BackupType *string `json:"type,omitempty"`
	Desc       *string `json:"desc,omitempty"`

	User string `json:"modified_user"`
}

func (opts BackupStrategyOptions) Valid() error {
	return nil
}

type BackupEndpoint struct {
	Enabled bool        `json:"enabled"`
	Name    string      `json:"name"`
	Id      string      `json:"id,omitempty"`
	SiteId  string      `json:"site_id"`
	Type    string      `json:"type"`
	Config  interface{} `json:"config"`
	Status  State       `json:"status,omitempty"`
	User    string      `json:"created_user"`
}

type BackupEndpointNfsConfig struct {
	NfsIP     string `json:"nfs_ip"`
	NfsOpts   string `json:"nfs_opts"`
	NfsSource string `json:"nfs_source"`
}

type BackupEndpointS3Config struct {
	S3Url    string `json:"s3_url"`
	S3Bucket string `json:"s3_bucket"`
	S3AcKey  string `json:"s3_access_key"`
	S3Secret string `json:"s3_secret_key"`
}

func (be BackupEndpoint) Valid() error {
	backupEpJsonSchemaOnce.Do(func() {
		var err error
		backupEpJsonSchema, err = GetJsonSchema("backupEndpoint.json")
		backupEpJsonSchemaLoaded = err == nil
	})

	if !backupEpJsonSchemaLoaded {
		return stderror.New("Failed to load json schema")
	}

	schemaLoader := gojsonschema.NewBytesLoader(backupEpJsonSchema)
	loader := gojsonschema.NewGoLoader(be)
	var errs []error

	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			errs = append(errs, stderror.Errorf("- %s\n", err))
		}
	}

	return utilerrors.NewAggregate(errs)
}
