package bankend

import (
	"context"
	"encoding/json"
	"fmt"
	stderror "github.com/pkg/errors"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"

	cron "github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/pkg/zone"
)

func NewBackupBankend(zone zone.ZoneInterface, ms modelSite, mbs modelStrategy, mbf modelBackupFile, mbe modelBackupEndpoint, apps modelApp) bankendBackup {
	return bankendBackup{
		mbs:   mbs,
		mbf:   mbf,
		mbe:   mbe,
		msite: ms,
		cron:  cron.New(),
		zone:  zoneIface{zone: zone},
		apps:  apps,
		waits: NewWaitTasks(),
	}
}

type bankendBackup struct {
	cron *cron.Cron

	mbs modelStrategy
	mbf modelBackupFile
	mbe modelBackupEndpoint

	msite modelSite
	apps  modelApp

	zone zoneIface

	waits *waitTasks
}

type modelStrategy interface {
	strategyGetter

	InsertStrategy(model.BackupStrategy) (string, error)
	UpdateStrategy(model.BackupStrategy) error
	DeleteStrategy(string, string) error
}

type strategyGetter interface {
	Lock(id string) (model.BackupStrategy, error)
	GetStrategy(id string) (model.BackupStrategy, error)
	ListStrategy(map[string]string) ([]model.BackupStrategy, error)
}

type backupFileGetter interface {
	GetFile(id string) (model.BackupFile, error)
	ListFiles(map[string]string) ([]model.BackupFile, error)
}

type modelBackupFile interface {
	backupFileGetter

	InsertFile(model.BackupFile) (string, error)
	UpdateFile(model.BackupFile) error
	BackupJobDone(model.BackupFile) error
	DeleteFile(string) error
}

type endpointGetter interface {
	GetEndpoint(string) (model.BackupEndpoint, error)
	ListEndpoint(map[string]string) ([]model.BackupEndpoint, error)
}

type modelBackupEndpoint interface {
	endpointGetter
	InsertEndpoint(model.BackupEndpoint) (string, error)
	UpdateEndpoint(model.BackupEndpoint) error
	DeleteEndpoint(string) error
}

func (b bankendBackup) ListBackupFiles(ctx context.Context, id, unit, app, site, user string) (api.BackupFilesResponse, error) {
	selector := make(map[string]string)
	if id != "" {
		selector["id"] = id
	}
	if unit != "" {
		selector["unit_id"] = unit
	}
	if app != "" {
		selector["app_id"] = app
	}
	if site != "" {
		selector["site_id"] = site
	}
	if user != "" {
		selector["created_user"] = user
	}

	list, err := b.mbf.ListFiles(selector)
	if err != nil {
		return nil, err
	}

	out := make([]api.BackupFile, len(list))

	for i := range list {
		var endpoint model.BackupEndpoint
		endpointType := "unkown"
		endpoint, err = b.mbe.GetEndpoint(list[i].EndpointId)
		if err == nil {
			endpointType = endpoint.Type
		}

		out[i] = api.BackupFile{
			Valid:        list[i].Status == model.BackupFileComplete,
			Size:         list[i].Size,
			ID:           list[i].ID,
			Name:         "",
			Status:       list[i].Status,
			Unit:         api.NewIDName(list[i].Unit, ""),
			App:          api.NewIDName(list[i].App, ""),
			Site:         api.NewIDName(list[i].Site, ""),
			Endpoint:     api.NewIDName(list[i].EndpointId, ""),
			EndpointType: endpointType,
			Path:         list[i].File,
			Type:         api.BackupType(list[i].Type),
			ExpiredAt:    api.Time(list[i].ExpiredAt),
			CreatedAt:    api.Time(list[i].CreatedAt),
			FinishedAt:   api.Time(list[i].FinishedAt),
			User:         list[i].CreatedUser,
		}
	}

	return out, nil
}

func (b bankendBackup) DeleteBackupFile(ctx context.Context, id, app string) error {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}
	if app != "" {
		selector["app_id"] = app
	}

	files, err := b.mbf.ListFiles(selector)
	if model.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	done := make(map[string]struct{}, len(files))
	wt := NewWaitTask(time.Second*30, func(err error) error {
		var errs []error

		for i := range files {
			if _, ok := done[files[i].ID]; ok {
				continue
			}

			files[i].Status = model.BackupFileDeleteFailed

			err := b.mbf.UpdateFile(files[i])
			if err != nil && !model.IsNotExist(err) {
				errs = append(errs, err)
				continue
			}
		}

		return utilerrors.NewAggregate(errs)
	})

	go wt.WithTimeout(time.Minute*5, func() (bool, error) {

		var errs []error

		for i := range files {

			if _, ok := done[files[i].ID]; ok {
				continue
			}

			ok, err := deleteBackupFile(files[i], b.mbf, b.zone, false)
			if ok && err == nil {
				done[files[i].ID] = struct{}{}
			} else {
				errs = append(errs, err)
			}
		}

		if len(done) == len(files) {
			return true, nil
		}

		return false, utilerrors.NewAggregate(errs)
	})

	return err
}

func deleteBackupFile(file model.BackupFile, mbf model.ModelBackupFile, zone zoneIface, async bool) (bool, error) {
	file.Status = model.BackupFileDeleting

	err := mbf.UpdateFile(file)
	if err != nil {
		if model.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	wt := NewWaitTask(time.Second*20, func(err error) error {
		if err != nil {
			file.Status = model.BackupFileDeleteFailed

			_err := mbf.UpdateFile(file)
			if _err != nil && !model.IsNotExist(_err) {
				err = fmt.Errorf("%v;%v", err, _err)
			}
		}

		return err
	})

	work := func() (bool, error) {

		iface, err := zone.siteInterface(file.Site)
		if err != nil {
			return false, err
		}

		job, err := iface.Jobs().Get(file.Namespace, file.Job)
		if err != nil && !errors.IsNotFound(err) {
			return false, err
		}
		if errors.IsNotFound(err) {
			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: file.Namespace,
					Name:      file.Job,
				},
			}
		}

		err = deleteJob(iface, job)
		if err != nil {
			return false, err
		}

		err = mbf.DeleteFile(file.ID)

		return err == nil, err
	}

	if !async {
		err := wt.WithTimeout(time.Second*30, work)

		return err == nil, err
	}

	go wt.WithTimeout(time.Minute*3, work)

	return true, nil
}

func deleteExpiredBackupFiles(mbf model.ModelBackupFile, zone zoneIface) error {
	files, err := mbf.ListFiles(map[string]string{})
	if err != nil {
		return err
	}

	var errs []error
	now := time.Now()

	for i := range files {

		if files[i].ExpiredAt.Before(now) {

			klog.Infof("Backup file %s is expired %s,delete...", files[i].ID, files[i].ExpiredAt)

			_, err := deleteBackupFile(files[i], mbf, zone, true)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (b bankendBackup) AddBackupStrategy(ctx context.Context, config api.BackupStrategyConfig) (api.ObjectResponse, error) {

	app, err := b.apps.Get(config.App)
	if err != nil {
		return api.ObjectResponse{}, err
	}

	if config.Unit != "" {

		exist := false
		for i := range app.Units {
			if app.Units[i].ID == config.Unit {
				exist = true
			}
		}

		if !exist {
			return api.ObjectResponse{}, fmt.Errorf("Not found Unit %s in App %s", config.Unit, app.ID)
		}
	}

	strategy := model.BackupStrategy{
		Enabled:    config.Enabled,
		Retention:  config.Retention,
		ID:         "",
		Name:       config.Name,
		Desc:       config.Desc,
		App:        app.ID,
		Unit:       config.Unit,
		EndpointId: config.EndpointId,
		Schedule:   config.Schedule,
		Role:       config.Role,
		Type:       string(config.BackupType),
		Tables:     model.NewSliceString(config.Tables),
		Editor: model.Editor{
			CreatedAt:   time.Now(),
			CreatedUser: config.User,
		},
	}

	if config.Once {
		// run backup job right now
		bs := b.newBackupStrategy(strategy)
		bs.app = app
		timestamp := strconv.Itoa(int(time.Now().Unix()))

		jr, err := bs.run(timestamp, nil)
		if err != nil {
			return api.ObjectResponse{}, err
		}

		return api.ObjectResponse{ID: jr.bf.ID}, err
	}

	strategy.ID, err = b.mbs.InsertStrategy(strategy)
	if err != nil {
		return api.ObjectResponse{}, err
	}

	_, err = b.cron.AddJob(config.Schedule, b.newBackupStrategy(strategy))

	return api.ObjectResponse{ID: strategy.ID}, err
}

func (b *bankendBackup) newBackupStrategy(strategy model.BackupStrategy) backupStrategy {
	return backupStrategy{
		apps:     b.apps,
		sites:    b.msite,
		getter:   b.mbs,
		mbf:      b.mbf,
		mbep:     b.mbe,
		zone:     b.zone,
		schedule: strategy.Schedule,
		strategy: strategy,
	}
}

func (b bankendBackup) SetBackupStrategy(ctx context.Context, id string, opts api.BackupStrategyOptions) error {
	strategy, err := b.mbs.GetStrategy(id)
	if err != nil {
		return err
	}

	bs := mergeBackupStrategy(strategy, opts)

	_, err = cron.ParseStandard(bs.Schedule)
	if err != nil {
		return err
	}

	err = b.mbs.UpdateStrategy(bs)
	if err != nil {
		return err
	}

	if opts.Schedule != nil && *opts.Schedule != strategy.Schedule {
		_, err = b.cron.AddJob(bs.Schedule, b.newBackupStrategy(bs))
	}

	return err
}

func mergeBackupStrategy(bs model.BackupStrategy, opts api.BackupStrategyOptions) model.BackupStrategy {

	if opts.Enabled != nil {
		bs.Enabled = *opts.Enabled
	}

	if opts.Retention != nil {
		bs.Retention = *opts.Retention
	}

	if opts.Schedule != nil {
		bs.Schedule = *opts.Schedule
	}

	if opts.App != nil {
		bs.App = *opts.App
	}
	if opts.Unit != nil {
		bs.Unit = *opts.Unit
	}

	if opts.BackupType != nil {
		bs.Type = *opts.BackupType
	}

	if opts.Desc != nil {
		bs.Desc = *opts.Desc
	}

	bs.ModifiedAt = time.Now()
	bs.ModifiedUser = opts.User

	return bs
}

func (b bankendBackup) ListBackupStrategy(ctx context.Context, id, unit, app string) (api.BackupStrategyResponse, error) {
	selector := make(map[string]string)
	if id != "" {
		selector["id"] = id
	}
	if unit != "" {
		selector["unit_id"] = unit
	}
	if app != "" {
		selector["app_id"] = app
	}

	list, err := b.mbs.ListStrategy(selector)
	if err != nil {
		return nil, err
	}

	out := make([]api.BackupStrategy, len(list))

	for i := range list {

		out[i] = api.BackupStrategy{
			ID:        list[i].ID,
			Name:      list[i].Name,
			Enabled:   list[i].Enabled,
			Retention: list[i].Retention,
			App:       list[i].App,
			Unit:      list[i].Unit,
			Schedule:  list[i].Schedule,
			Desc:      list[i].Desc,
			Role:      list[i].Role,
			Type:      api.BackupType(list[i].Type),
			Tables:    list[i].Tables.Strings(),
			Created:   api.NewEditor(list[i].CreatedUser, list[i].CreatedAt),
			Modified:  api.NewEditor(list[i].ModifiedUser, list[i].ModifiedAt),
		}
	}

	return out, nil
}

func (b bankendBackup) DeleteBackupStrategy(ctx context.Context, id, app string) error {

	return b.mbs.DeleteStrategy(id, app)
}

func (b bankendBackup) CronStartAndRestore() error {
	b.cron.Start()

	list, err := b.mbs.ListStrategy(map[string]string{})
	if model.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var errs []error

	for i := range list {
		bs := b.newBackupStrategy(list[i])

		_, err := b.cron.AddJob(bs.schedule, bs)
		if err != nil {
			errs = append(errs, fmt.Errorf("Strategy %s '%s' error:%s", bs.strategy.ID, bs.schedule, err))
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (b bankendBackup) preValidateEndpoint(endpoint api.BackupEndpoint, id string) (string, error) {
	//check name dup
	epSelector := make(map[string]string)
	epSelector["name"] = strings.TrimSpace(endpoint.Name)
	endpoints, err := b.mbe.ListEndpoint(epSelector)
	if err != nil {
		return "", stderror.New("error query DB")
	} else {
		if len(endpoints) > 0 {
			if id != "" && endpoints[0].ID != id {
				return "", stderror.New("name is duplicated")
			}
		}
	}

	//check cluster_id is valid
	siteSelector := make(map[string]string)
	siteSelector["id"] = endpoint.SiteId
	sites, err := b.msite.List(siteSelector)
	if err != nil {
		return "", stderror.New("error query DB")
	} else {
		if len(sites) != 1 {
			return "", stderror.New("site id is not found")
		}
	}

	return sites[0].ID, nil
}

func (b bankendBackup) AddEndpoint(ctx context.Context, endpoint api.BackupEndpoint) (api.BackupEndpoint, error) {

	var err error
	createdAt := time.Now()

	siteId, err := b.preValidateEndpoint(endpoint, "")
	if err != nil {
		return api.BackupEndpoint{}, err
	}

	modelEndpoint := model.BackupEndpoint{
		Enabled: endpoint.Enabled,
		ID:      "",
		Name:    strings.TrimSpace(endpoint.Name),
		SiteId:  strings.ToLower(endpoint.SiteId),
		Type:    strings.ToLower(endpoint.Type),
		Config:  "",
		Editor: model.Editor{
			CreatedAt:    createdAt,
			CreatedUser:  endpoint.User,
			ModifiedAt:   createdAt,
			ModifiedUser: endpoint.User,
		},
	}

	// the validation will be done by jsonschema when request is received
	config, _ := json.Marshal(endpoint.Config)
	modelEndpoint.Config = string(config)

	endpoint.Id, err = b.mbe.InsertEndpoint(modelEndpoint)
	if err != nil {
		return api.BackupEndpoint{}, err
	}

	if modelEndpoint.Type == "nfs" {
		err = b.AddNfsEndpointDeployment(siteId, endpoint)
		if err != nil {
			_ = b.mbe.DeleteEndpoint(endpoint.Id)
			return api.BackupEndpoint{}, err
		}
	}

	return endpoint, err
}

func (b bankendBackup) UpdateEndpoint(ctx context.Context, id string, endpoint api.BackupEndpoint) (api.BackupEndpoint, error) {

	var err error
	modifiedAt := time.Now()

	_, err = b.preValidateEndpoint(endpoint, id)
	if err != nil {
		return api.BackupEndpoint{}, err
	}

	modelEndpoint := model.BackupEndpoint{
		Enabled: endpoint.Enabled,
		ID:      id,
		Name:    strings.TrimSpace(endpoint.Name),
		SiteId:  strings.ToLower(endpoint.SiteId),
		Type:    strings.ToLower(endpoint.Type),
		Config:  "",
		Editor: model.Editor{
			ModifiedAt:   modifiedAt,
			ModifiedUser: endpoint.User,
		},
	}

	// the validation will be done by jsonschema when request is received
	config, _ := json.Marshal(endpoint.Config)
	modelEndpoint.Config = string(config)

	err = b.mbe.UpdateEndpoint(modelEndpoint)
	if err != nil {
		return api.BackupEndpoint{}, err
	}

	return endpoint, err
}

func (b bankendBackup) DeleteEndpoint(ctx context.Context, id string) error {

	var err error

	endpoint, err := b.mbe.GetEndpoint(id)
	if err != nil {
		return err
	}

	if endpoint.Type == "nfs" {
		err = b.DeleteNfsEndpointDeployment(endpoint.SiteId, endpoint.ID)
		if err != nil {
			return err
		}
	}

	err = b.mbe.DeleteEndpoint(id)
	if err != nil {
		return err
	}

	return nil
}

func (b bankendBackup) GetEndpoint(ctx context.Context, id string) (api.BackupEndpoint, error) {

	var err error
	var ret api.BackupEndpoint

	endpoint, err := b.mbe.GetEndpoint(id)
	if err != nil {
		return api.BackupEndpoint{}, err
	}

	if endpoint.Type == "nfs" {
		var nfsConfig api.BackupEndpointNfsConfig
		_ = json.Unmarshal([]byte(endpoint.Config), &nfsConfig)
		status, _ := b.GetNfsEndpointDeploymentStatus(endpoint.SiteId, endpoint.ID)

		ret = api.BackupEndpoint{
			Enabled: endpoint.Enabled,
			Name:    endpoint.Name,
			Id:      endpoint.ID,
			SiteId:  endpoint.SiteId,
			Type:    endpoint.Type,
			Config:  nfsConfig,
			Status:  status,
			User:    endpoint.CreatedUser,
		}
	} else {
		var s3Config api.BackupEndpointS3Config
		_ = json.Unmarshal([]byte(endpoint.Config), &s3Config)

		ret = api.BackupEndpoint{
			Enabled: endpoint.Enabled,
			Name:    endpoint.Name,
			Id:      endpoint.ID,
			SiteId:  endpoint.SiteId,
			Type:    endpoint.Type,
			Config:  s3Config,
			User:    endpoint.CreatedUser,
		}
	}

	return ret, err
}

func (b bankendBackup) ListEndpoint(ctx context.Context, siteId, endpointType string) ([]api.BackupEndpoint, error) {

	var err error
	selector := make(map[string]string)

	if siteId != "" {
		selector["site_id"] = siteId
	}

	if endpointType != "" {
		selector["type"] = endpointType
	}

	endpoints, err := b.mbe.ListEndpoint(selector)
	if err != nil {
		return []api.BackupEndpoint{}, err
	}

	var list []api.BackupEndpoint
	for _, endpoint := range endpoints {
		if endpoint.Type == "nfs" {
			var nfsConfig api.BackupEndpointNfsConfig
			_ = json.Unmarshal([]byte(endpoint.Config), &nfsConfig)
			status, _ := b.GetNfsEndpointDeploymentStatus(endpoint.SiteId, endpoint.ID)

			list = append(list, api.BackupEndpoint{
				Enabled: endpoint.Enabled,
				Name:    endpoint.Name,
				Id:      endpoint.ID,
				SiteId:  endpoint.SiteId,
				Type:    endpoint.Type,
				Config:  nfsConfig,
				Status:  status,
				User:    endpoint.CreatedUser,
			})
		} else {
			var s3Config api.BackupEndpointS3Config
			_ = json.Unmarshal([]byte(endpoint.Config), &s3Config)

			list = append(list, api.BackupEndpoint{
				Enabled: endpoint.Enabled,
				Name:    endpoint.Name,
				Id:      endpoint.ID,
				SiteId:  endpoint.SiteId,
				Type:    endpoint.Type,
				Config:  s3Config,
				User:    endpoint.CreatedUser,
			})
		}
	}

	return list, err
}
