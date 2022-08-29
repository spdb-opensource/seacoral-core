package bankend

import (
	"context"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/dashboard"
	"github.com/upmio/dbscale-kube/pkg/server"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	dashboardrt "github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/dashboard"
	siteRouters "github.com/upmio/dbscale-kube/cluster_manager/apiserver/routers/site"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/zone"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
)

const (
	siteStatePassing  = "passing"
	siteStateCritical = "critical"
	siteStateNotExist = "not exist"
)

func NewSiteBankend(port string, zone zone.ZoneInterface,
	ms modelSite,
	clusters clusterGetter,
	storages storageGetter,
	srv *server.Server) *bankendSite {
	return &bankendSite{
		ms:       ms,
		zone:     zone,
		execPort: port,

		clusters: clusters,
		storages: storages,
		srv:      srv,
	}
}

type modelSite interface {
	Insert(model.Site) (string, error)
	Update(model.Site) error
	Delete(name string) error

	siteGetter
}

type siteGetter interface {
	Get(id string) (model.Site, error)
	List(selector map[string]string) ([]model.Site, error)
}

type bankendSite struct {
	execPort string
	ms       modelSite
	zone     zone.ZoneInterface

	clusters clusterGetter
	storages storageGetter
	srv      *server.Server
}

func siteVersion(site zone.Site) (string, string, error) {
	iface, err := site.SiteInterface()
	if err != nil {
		return siteStateCritical, "", err
	}

	v, err := iface.ServerVersion()
	if err != nil {
		return siteStateCritical, "", err
	}

	return siteStatePassing, Version(v), nil
}

func (b *bankendSite) siteVersion(id string) (string, string, error) {
	site, err := b.zone.GetSite(id)
	if err != nil {
		return siteStateNotExist, "", err
	}

	return siteVersion(site)
}

func (b *bankendSite) InitDashboards() error {
	selector := make(map[string]string)
	list, err := b.ms.List(selector)

	if err != nil {
		return err
	}

	for _, site := range list {
		dashboardApi, err := dashboard.NewDashboardAPIHandler(siteRouters.ConfigPath(site.Domain, site.Port), site.Domain, strconv.Itoa(site.Port))
		if err != nil {
			return err
		}

		dashboardrt.RegisterDashboardRoute(site.Domain, dashboardApi, b.srv)
	}

	return nil
}

func (b *bankendSite) Add(ctx context.Context, config api.SiteConfig) (api.Site, error) {
	selector := make(map[string]string)
	selector["name"] = config.Name

	sites, err := b.ms.List(selector)
	if err != nil {
		return api.Site{}, err
	} else if len(sites) > 0 {
		return api.Site{}, stderror.Errorf("site name %s already exists", config.Name)
	}

	ms := convertToModelSite(config)

	if config.Path != "" {

		txt, err := ioutil.ReadFile(config.Path)
		if err != nil {
			return api.Site{}, err
		}

		ms.Config = string(txt)
	}

	out := convertToSite(ms)

	if config.Type == api.KubernetesSite {

		addr := net.JoinHostPort(config.Domain, b.execPort)

		site := zone.NewK8sSite(ms.ID, config.Domain, addr, config.Path, config.Port)

		err := b.zone.AddSite(site)
		if err != nil {
			return out, err
		}

		out.State, out.Version, err = siteVersion(site)
		if err != nil {
			return out, err
		}
	}

	id, err := b.ms.Insert(ms)
	out.ID = id

	dashboardApi, err := dashboard.NewDashboardAPIHandler(config.Path, config.Domain, strconv.Itoa(config.Port))
	if err != nil {
		return api.Site{}, err
	}

	dashboardrt.RegisterDashboardRoute(config.Domain, dashboardApi, b.srv)

	return out, err
}

func Version(v *version.Info) string {
	if v == nil {
		return "<unknown>"
	}

	return fmt.Sprintf("%s-%s %s %s", v.Major, v.Minor, v.GitVersion, v.GitCommit)
}

func (b *bankendSite) List(ctx context.Context, id, name string) ([]api.Site, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}
	if name != "" {
		selector["name"] = name
	}

	list, err := b.ms.List(selector)
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	once := new(sync.Once)
	sites := make([]api.Site, len(list))

	for i := range list {
		sites[i] = convertToSite(list[i])

		sites[i].State, sites[i].Version, err = b.siteVersion(sites[i].ID)
		if err != nil {
			once.Do(func() {
				b.restoreSites(list)
				sites[i].State, sites[i].Version, err = b.siteVersion(sites[i].ID)
			})
		}
	}

	return sites, nil
}

func (b *bankendSite) Set(ctx context.Context, id string, opts api.SiteOptions) error {
	ms, err := b.ms.Get(id)
	if err != nil {
		return err
	}

	ms, update := mergeSite(ms, opts)
	if update {
		txt, err := ioutil.ReadFile(opts.Path)
		if err != nil {
			return err
		}

		ms.Config = string(txt)
	}

	if update && ms.Type == api.KubernetesSite {

		addr := net.JoinHostPort(ms.Domain, b.execPort)

		site := zone.NewK8sSite(ms.ID, ms.Domain, addr, opts.Path, ms.Port)

		err = b.zone.AddSite(site)
		if err != nil {
			return err
		}
	}

	err = b.ms.Update(ms)
	if err != nil {
		return err
	}

	return nil
}

func mergeSite(ms model.Site, opts api.SiteOptions) (model.Site, bool) {
	update := false

	if opts.Name != nil {
		ms.Name = *opts.Name
	}

	if opts.Domain != nil {
		ms.Domain = *opts.Domain
		update = true
	}

	if opts.Port != nil {
		ms.Port = *opts.Port
		update = true
	}

	if opts.Region != nil {
		ms.Region = *opts.Region
	}

	if opts.Desc != nil {
		ms.Desc = *opts.Desc
	}

	ms.ModifiedAt = time.Now()
	if opts.User != "" {
		ms.ModifiedUser = opts.User
	}

	return ms, update
}

func (b *bankendSite) Delete(ctx context.Context, id string) error {
	selector := make(map[string]string)
	selector["site_id"] = id

	clusterOut, err := b.clusters.List(selector)
	if err != nil {
		klog.Errorf("List clusters err: %s", err)
		return fmt.Errorf("List clusters err: %s", err)
	}
	if clusterOut != nil && len(clusterOut) != 0 {
		klog.Info("cluster:", clusterOut)
		return fmt.Errorf("Please delete clusters before delete site.")
	}

	storageOut, err := b.storages.List(selector)
	if err != nil {
		klog.Errorf("List storages err: %s", err)
		return fmt.Errorf("List storages err: %s", err)
	}
	if storageOut != nil && len(storageOut) != 0 {
		klog.Info("storage:", storageOut)
		return fmt.Errorf("Please delete storages before delete site.")
	}

	site, err := b.ms.Get(id)
	if model.IsNotExist(err) {
		b.zone.RemoveSite(id)

		return nil
	}
	if err != nil {
		return err
	}

	err = b.zone.RemoveSite(site.ID)
	if err != nil {
		return err
	}

	err = b.ms.Delete(site.ID)
	if err != nil && !model.IsNotExist(err) {
		return err
	}

	return nil
}

func (b *bankendSite) RestoreSites() error {
	sites, err := b.ms.List(map[string]string{})
	if err != nil {
		return err
	}

	return b.restoreSites(sites)
}

func (b *bankendSite) restoreSites(sites []model.Site) error {
	errs := []error{}

	for i := range sites {

		if sites[i].Type != api.KubernetesSite {
			continue
		}

		file, err := ioutil.TempFile("", "kubeconfig")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defer func(file *os.File) {
			file.Close()
			os.Remove(file.Name())
		}(file)

		_, err = file.WriteString(sites[i].Config)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		addr := net.JoinHostPort(sites[i].Domain, b.execPort)

		site := zone.NewK8sSite(sites[i].ID, sites[i].Domain, addr, file.Name(), sites[i].Port)

		err = b.zone.AddSite(site)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func convertToSite(ms model.Site) api.Site {
	return api.Site{
		ID:            ms.ID,
		Name:          ms.Name,
		Type:          ms.Type,
		Domain:        ms.Domain,
		ImageRegistry: ms.ImageRegistry,
		ProjectName:   ms.ProjectName,
		NetworkMode:   ms.NetworkMode,
		Port:          ms.Port,
		Desc:          ms.Desc,
		Region:        ms.Region,
		Created:       api.NewEditor(ms.CreatedUser, ms.CreatedAt),
		Modified:      api.NewEditor(ms.ModifiedUser, ms.ModifiedAt),
	}
}

func convertToModelSite(config api.SiteConfig) model.Site {
	return model.Site{
		ID:            utils.NewUUID(),
		Name:          config.Name,
		Type:          config.Type,
		Desc:          config.Desc,
		Domain:        config.Domain,
		Port:          config.Port,
		Region:        config.Region,
		ImageRegistry: config.ImageRegistry,
		ProjectName:   config.ProjectName,
		NetworkMode:   config.NetworkMode,
		Editor:        newCreateEditor(config.User),
	}
}
