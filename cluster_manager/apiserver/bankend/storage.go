package bankend

import (
	"context"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	cryptoutil "github.com/upmio/dbscale-kube/pkg/utils/crypto"
	"github.com/upmio/dbscale-kube/pkg/zone"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func NewStorageBankend(
	zone zone.ZoneInterface,
	m storageModel,
	getter siteGetter,
	key string) *bankendStorage {
	return &bankendStorage{
		key:    key,
		m:      m,
		getter: getter,
		zone:   zone,
		waits:  NewWaitTasks(),
	}
}

type bankendStorage struct {
	key string

	m      storageModel
	getter siteGetter

	zone zone.ZoneInterface

	waits *waitTasks
}

type storageModel interface {
	Insert(model.RemoteStorage) (string, string, error)
	InsertRemoteStorageTask(model.RemoteStorage, string) (string, error)
	Update(model.RemoteStorage) (string, error)
	UpdateRemoteStorageTask(*model.RemoteStorage, model.Task) error
	Delete(name string) error

	storageGetter

	storagePoolModel
}

type storageGetter interface {
	Get(name string) (model.RemoteStorage, error)
	List(selector map[string]string) ([]model.RemoteStorage, error)
}

type storagePoolModel interface {
	poolGetter

	InsertPool(pool model.StoragePool) (string, string, error)
	InsertPoolTask(model.StoragePool, string) (string, error)
	UpdatePool(pool model.StoragePool) error
	UpdatePoolTask(pool *model.StoragePool, tk model.Task) error
	DeletePool(name string) error
}

type poolGetter interface {
	GetPool(name string) (model.StoragePool, error)
	ListPools(selector map[string]string) ([]model.StoragePool, error)
}

func (b *bankendStorage) Add(ctx context.Context, config api.RemoteStorageConfig) (api.RemoteStorage, error) {
	site, err := b.getter.Get(config.Site)
	if err != nil {
		return api.RemoteStorage{}, err
	}

	config.Site = site.ID
	config.Auth.Password = cryptoutil.AesEncrypto(config.Auth.Password, b.key)

	rs := convertToModelRemoteStorage(config, site)

	id, task, err := b.m.Insert(rs)
	rs.ID = id
	rs.Task.ID = task

	sys := convertToSanSystemV1alpha1(config)
	sys.Name = rs.ObjectName()

	wt := b.waits.NewWaitTask(rs.ID, time.Second*15, func(err error) error {
		tk := taskUpdate(task, err)
		if err == nil {
			rs.Enabled = config.Enabled
			return b.m.UpdateRemoteStorageTask(&rs, tk)
		}

		return b.m.UpdateRemoteStorageTask(nil, tk)
	})
	go wt.WithTimeout(time.Minute*2, func() (done bool, err error) {

		iface, err := b.zone.SanSystemInterface(config.Site)
		if err != nil {
			return false, err
		}

		clone, err := iface.Get(sys.Name)
		if errors.IsNotFound(err) {
			clone, err = iface.Create(sys)
		}
		if err != nil {
			return false, err
		}

		return clone.Status.Connected, nil
	})

	return convertToRemoteStorageAPI(rs, sys), err
}

func convertToSanSystemV1alpha1(req api.RemoteStorageConfig) *v1alpha1.SanSystem {

	return &v1alpha1.SanSystem{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				labelSite: req.Site,
			},
		},
		Spec: v1alpha1.SanSystemSpec{
			Auth: v1alpha1.Auth{
				IP:         req.Auth.IP.String(),
				Port:       req.Auth.Port,
				User:       req.Auth.User,
				Password:   req.Auth.Password,
				Vstorename: req.Auth.Vstorename,
				Vendor:     req.Vendor + "-" + req.Model,
			},
			//		StoragePoolList: pools,
		},
	}
}

func convertToModelRemoteStorage(req api.RemoteStorageConfig, site model.Site) model.RemoteStorage {

	rs := model.RemoteStorage{
		// Enabled: req.Enabled,
		Name:   req.Name,
		SiteID: site.ID,
		Site: model.SiteBrief{
			ID:   site.ID,
			Name: site.Name,
		},
		Vendor: req.Vendor,
		Model:  req.Model,
		Type:   req.Type,
		Desc:   req.Desc,
		Editor: newCreateEditor(req.User),
		Auth: model.Auth{
			Port:            req.Auth.Port,
			RemoteStorageID: req.Name,
			IP:              req.Auth.IP.String(),
			User:            req.Auth.User,
			Password:        req.Auth.Password,
			Vstorename:      req.Auth.Vstorename,
		},
	}

	return rs
}

func convertToRemoteStorageAPI(rs model.RemoteStorage, san *v1alpha1.SanSystem) api.RemoteStorage {
	storage := api.RemoteStorage{
		Enabled: rs.Enabled,
		ID:      rs.ID,
		Name:    rs.Name,
		Desc:    rs.Desc,
		Site:    api.NewIDName(rs.SiteID, rs.Site.Name),
		Type:    rs.Type,
		Model:   rs.Model,
		Vendor:  rs.Vendor,
		Auth: api.Auth{
			Port:       rs.Auth.Port,
			IP:         api.IP(rs.Auth.IP),
			User:       rs.Auth.User,
			Vstorename: rs.Auth.Vstorename,
		},
		Task:     convertToTaskBrief(rs.Task),
		Created:  api.NewEditor(rs.CreatedUser, rs.CreatedAt),
		Modified: api.NewEditor(rs.ModifiedUser, rs.ModifiedAt),
	}

	if san != nil {
		storage.ResourceStatus = api.ResourceStatus{
			Used:     int64(san.Status.Total - san.Status.Free),
			Capacity: int64(san.Status.Total),
		}
	}

	state := "passing"
	if san == nil {
		state = "unknown"
	} else if !san.Status.Connected {
		state = "critical"
	}

	storage.Status = state

	return storage
}

func (b *bankendStorage) List(ctx context.Context, id, name, site, enabled string) ([]api.RemoteStorage, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}
	if name != "" {
		selector["name"] = name
	}
	if site != "" {
		selector["site_id"] = site
	}
	if enabled != "" {
		selector[labelEnabled] = boolValue(enabled)
	}

	list, err := b.m.List(selector)
	if err != nil {
		return nil, err
	}

	storages := make([]api.RemoteStorage, len(list))
	errs := make([]error, 0, len(list))

	for i := range list {

		iface, err := b.zone.SanSystemInterface(list[i].SiteID)
		if err != nil {
			errs = append(errs, err)

			storages[i] = convertToRemoteStorageAPI(list[i], nil)
			continue
		}

		sys, err := iface.Get(list[i].ObjectName())
		if err != nil {
			errs = append(errs, err)
		}

		storages[i] = convertToRemoteStorageAPI(list[i], sys)
	}

	return storages, utilerrors.NewAggregate(errs)
}

func (b *bankendStorage) Set(ctx context.Context, id string, opts api.RemoteStorageOptions) (api.RemoteStorage, error) {
	rs, err := b.m.Get(id)
	if err != nil {
		return api.RemoteStorage{}, err
	}

	if opts.Auth.Password != nil {
		pwd := cryptoutil.AesEncrypto(*opts.Auth.Password, b.key)
		opts.Auth.Password = &pwd
	}

	rs, _ = mergeRemoteStorage(rs, opts)

	task, err := b.m.Update(rs)
	if err != nil {
		return api.RemoteStorage{}, err
	}

	rs.Task = model.Task{
		ID: task,
	}

	var sys *v1alpha1.SanSystem
	wt := b.waits.NewWaitTask(rs.ID, time.Second*15, func(err error) error {
		tk := taskUpdate(task, err)

		if err != nil {
			rs.Enabled = false
			return b.m.UpdateRemoteStorageTask(&rs, tk)
		}

		return b.m.UpdateRemoteStorageTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*2, func() (bool, error) {

		iface, err := b.zone.SanSystemInterface(rs.SiteID)
		if err != nil {
			return false, err
		}

		sys, err = iface.Get(rs.ObjectName())
		if err != nil {
			return false, err
		}

		if sys.Spec.Auth.IP != rs.Auth.IP ||
			sys.Spec.Auth.Port != rs.Auth.Port ||
			sys.Spec.Auth.User != rs.Auth.User ||
			// 备注：这里需要修改/pkg/apis/san/v1alpha1/types.go，暂时不修改
			// sys.Spec.Auth.Vstorename != rs.Auth.Vstorename ||
			sys.Spec.Auth.Password != rs.Auth.Password {

			sys = sys.DeepCopy()
			sys.Spec.Auth.IP = rs.Auth.IP
			sys.Spec.Auth.Port = rs.Auth.Port
			sys.Spec.Auth.User = rs.Auth.User
			sys.Spec.Auth.Password = rs.Auth.Password

			sys, err = iface.Update(sys)
			if err != nil {
				return false, err
			}
		}

		if sys.Status.Disable == rs.Enabled {

			sys = sys.DeepCopy()
			sys.Status.Disable = !rs.Enabled

			sys, err = iface.UpdateStatus(sys)
			if err != nil {
				return false, err
			}
		}

		return sys.Status.Connected, nil
	})

	return convertToRemoteStorageAPI(rs, sys), nil
}

func mergeRemoteStorage(rs model.RemoteStorage, opts api.RemoteStorageOptions) (model.RemoteStorage, bool) {
	update := false

	if opts.Name != nil && rs.Name != *opts.Name {
		rs.Name = *opts.Name
	}

	if opts.Desc != nil && rs.Desc != *opts.Desc {
		rs.Desc = *opts.Desc
	}

	if opts.Enabled != nil && rs.Enabled != *opts.Enabled {
		rs.Enabled = *opts.Enabled
		update = true
	}

	if opts.Auth.Port != nil && rs.Auth.Port != *opts.Auth.Port {
		update = true
		rs.Auth.Port = *opts.Auth.Port
	}
	if opts.Auth.IP != nil && rs.Auth.IP != *opts.Auth.IP {
		update = true
		rs.Auth.IP = *opts.Auth.IP
	}
	if opts.Auth.User != nil && rs.Auth.User != *opts.Auth.User {
		update = true
		rs.Auth.User = *opts.Auth.User
	}
	if opts.Auth.Password != nil && rs.Auth.Password != *opts.Auth.Password {
		update = true
		rs.Auth.Password = *opts.Auth.Password
	}
	if opts.Auth.Vstorename != nil && rs.Auth.Vstorename != *opts.Auth.Vstorename {
		update = true
		rs.Auth.Vstorename = *opts.Auth.Vstorename
	}

	if opts.User != "" {
		rs.ModifiedUser = opts.User
	}

	rs.ModifiedAt = time.Now()

	return rs, update
}

func (b *bankendStorage) Delete(ctx context.Context, id string) (api.TaskObjectResponse, error) {
	rs, err := b.m.Get(id)
	if model.IsNotExist(err) {
		return api.TaskObjectResponse{}, nil
	}
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	if rs.ID == "" {
		rs.ID = id
	}

	task, err := b.m.InsertRemoteStorageTask(rs, model.ActionRemoteStorageDelete)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	b.waits.Delete(rs.ID)

	wt := NewWaitTask(time.Second*10, func(err error) error {
		if err == nil {
			err = b.m.Delete(rs.ID)
		}

		tk := taskUpdate(task, err)

		return b.m.UpdateRemoteStorageTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute, func() (bool, error) {

		iface, err := b.zone.SanSystemInterface(rs.SiteID)
		if err != nil {
			return false, err
		}

		err = iface.Delete(rs.ObjectName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}

		return err == nil, err
	})

	return api.TaskObjectResponse{
		ObjectID:   rs.ID,
		ObjectName: rs.Name,
		TaskID:     task,
	}, nil
}

func (b *bankendStorage) AddPool(ctx context.Context, storage string, config api.RemoteStoragePoolConfig) (api.RemoteStoragePool, error) {
	rs, err := b.m.Get(storage)
	if err != nil {
		return api.RemoteStoragePool{}, err
	}

	config.Storage = rs.ID

	pool := convertToModelStoragePool(config)

	id, task, err := b.m.InsertPool(pool)
	if err != nil {
		return api.RemoteStoragePool{}, err
	}

	pool.ID = id
	pool.Task.ID = task

	wt := NewWaitTask(time.Second*15, func(err error) error {
		tk := taskUpdate(task, err)
		if err == nil {
			pool.Enabled = config.Enabled

			return b.m.UpdatePoolTask(&pool, tk)
		}

		return b.m.UpdatePoolTask(nil, tk)
	})
	go wt.WithTimeout(time.Minute, func() (bool, error) {

		iface, err := b.zone.SanSystemInterface(rs.SiteID)
		if err != nil {
			return false, err
		}

		sys, err := iface.Get(rs.ObjectName())
		if err != nil {
			return false, err
		}

		pools := sys.Spec.StoragePoolList
		for i := range pools {
			if pools[i].Name == pool.ObjectName() {
				return true, nil
			}
		}

		pools = append(pools, v1alpha1.StoragePoolWithLevel{
			Name:    pool.ObjectName(),
			Level:   v1alpha1.Level(config.Performance),
			Enabled: config.Enabled,
		})

		sys = sys.DeepCopy()
		sys.Spec.StoragePoolList = pools

		sys, err = iface.Update(sys)
		if err != nil {
			return false, err
		}

		pools = sys.Spec.StoragePoolList
		for i := range pools {
			if pools[i].Name == pool.ObjectName() {
				return true, nil
			}
		}

		return false, nil
	})

	return convertToStoragePoolAPI(pool, nil), err
}

func convertToModelStoragePool(config api.RemoteStoragePoolConfig) model.StoragePool {
	return model.StoragePool{
		// Enabled:         config.Enabled,
		Name:            config.Name,
		Performance:     string(config.Performance),
		Desc:            config.Desc,
		RemoteStorageID: config.Storage,
		NativeID:        config.Native,
		Editor:          newCreateEditor(config.User),
	}
}

func convertToStoragePoolAPI(pool model.StoragePool, sys *v1alpha1.SanSystem) api.RemoteStoragePool {
	rsp := api.RemoteStoragePool{
		Enabled:     pool.Enabled,
		ID:          pool.ID,
		Name:        pool.Name,
		Storage:     api.NewIDName(pool.RemoteStorageID, pool.RemoteStorageName),
		Native:      pool.NativeID,
		Desc:        pool.Desc,
		Performance: api.Performance(pool.Performance),
		// 	ResourceStatus
		Task:     convertToTaskBrief(pool.Task),
		Created:  api.NewEditor(pool.CreatedUser, pool.CreatedAt),
		Modified: api.NewEditor(pool.ModifiedUser, pool.ModifiedAt),
	}

	if sys != nil {
		for i := range sys.Status.Pools {
			if sys.Status.Pools[i].Name == pool.ObjectName() {
				rsp.ResourceStatus = api.ResourceStatus{
					Used:     int64(sys.Status.Pools[i].Total - sys.Status.Pools[i].Free),
					Capacity: int64(sys.Status.Pools[i].Total),
				}
			}
		}
	}

	return rsp
}

func (b *bankendStorage) ListPools(ctx context.Context, storage string, id, name string) ([]api.RemoteStoragePool, error) {
	rs, err := b.m.Get(storage)
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	selector := make(map[string]string)
	selector["remote_storage_id"] = rs.ID
	if id != "" {
		selector["id"] = id
	}
	if name != "" {
		selector["name"] = name
	}

	list, err := b.m.ListPools(selector)
	if err != nil {
		return nil, err
	}

	var sys *v1alpha1.SanSystem

	iface, err := b.zone.SanSystemInterface(rs.SiteID)
	if err == nil {
		sys, err = iface.Get(rs.ObjectName())
	}

	pools := make([]api.RemoteStoragePool, len(list))

	for i := range list {
		pools[i] = convertToStoragePoolAPI(list[i], sys)
	}

	return pools, err
}

func (b *bankendStorage) SetPool(ctx context.Context, storage string, id string, opts api.RemoteStoragePoolOptions) (api.RemoteStoragePool, error) {
	pool, err := b.m.GetPool(id)
	if err != nil {
		return api.RemoteStoragePool{}, err
	}

	pool = mergeStoragePool(pool, opts)
	if opts.Enabled != nil {

		wt := b.waits.NewWaitTask(pool.ID, time.Second*10, func(err error) error {
			return err
		})
		go wt.WithTimeout(time.Minute, func() (bool, error) {

			rs, err := b.m.Get(storage)
			if err != nil {
				return false, err
			}

			iface, err := b.zone.SanSystemInterface(rs.SiteID)
			if err != nil {
				return false, err
			}

			sys, err := iface.Get(rs.ObjectName())
			if err != nil {
				return false, err
			}

			for i := range sys.Spec.StoragePoolList {

				if sys.Spec.StoragePoolList[i].Name == pool.ObjectName() &&
					sys.Spec.StoragePoolList[i].Enabled == *opts.Enabled {

					return true, nil
				}

				if sys.Spec.StoragePoolList[i].Name == pool.ObjectName() &&
					sys.Spec.StoragePoolList[i].Enabled != *opts.Enabled {

					sys = sys.DeepCopy()
					sys.Spec.StoragePoolList[i].Enabled = *opts.Enabled

					sys, err = iface.Update(sys)
					if err != nil {
						return false, err
					}

					if sys.Spec.StoragePoolList[i].Name == pool.ObjectName() &&
						sys.Spec.StoragePoolList[i].Enabled == *opts.Enabled {

						return true, nil
					}
				}
			}

			return false, nil
		})
	}

	err = b.m.UpdatePool(pool)

	return convertToStoragePoolAPI(pool, nil), err
}

func mergeStoragePool(pool model.StoragePool, opts api.RemoteStoragePoolOptions) model.StoragePool {
	if opts.Enabled != nil {
		pool.Enabled = *opts.Enabled
	}

	if opts.Name != nil {
		pool.Name = *opts.Name
	}

	if opts.Desc != nil {
		pool.Desc = *opts.Desc
	}

	if opts.User != "" {
		pool.ModifiedUser = opts.User
	}

	pool.ModifiedAt = time.Now()

	return pool
}

func (b *bankendStorage) DeletePool(ctx context.Context, storage string, id string) (api.TaskObjectResponse, error) {
	rs, err := b.m.Get(storage)
	if model.IsNotExist(err) {
		return api.TaskObjectResponse{}, nil
	}
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	pool, err := b.m.GetPool(id)
	if model.IsNotExist(err) {
		return api.TaskObjectResponse{}, nil
	}
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	task, err := b.m.InsertPoolTask(pool, model.ActionRemoteStoragePoolDelete)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	b.waits.Delete(pool.ID)

	wt := NewWaitTask(time.Second*10, func(err error) error {
		if err == nil {
			err = b.m.DeletePool(pool.ID)
		}

		tk := taskUpdate(task, err)

		return b.m.UpdatePoolTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute, func() (bool, error) {

		iface, err := b.zone.SanSystemInterface(rs.SiteID)
		if err != nil {
			return false, err
		}

		sys, err := iface.Get(rs.ObjectName())
		if err != nil {
			return false, err
		}

		pools := make([]v1alpha1.StoragePoolWithLevel, 0, len(sys.Spec.StoragePoolList))

		exist := false

		for i := range sys.Spec.StoragePoolList {
			if sys.Spec.StoragePoolList[i].Name != pool.ObjectName() {
				pools = append(pools, sys.Spec.StoragePoolList[i])
			} else {
				exist = true
			}
		}

		if !exist {
			return true, nil
		}

		if len(pools) < cap(pools) {

			sys = sys.DeepCopy()
			sys.Spec.StoragePoolList = pools

			sys, err = iface.Update(sys)
			if err != nil {
				return false, err
			}
		}

		return false, nil
	})

	return api.TaskObjectResponse{
		ObjectID:   pool.ID,
		ObjectName: pool.Name,
		TaskID:     task,
	}, nil
}
