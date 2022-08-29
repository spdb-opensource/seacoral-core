package bankend

import (
	"context"
	"fmt"
	stderror "github.com/pkg/errors"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"k8s.io/klog/v2"
)

func NewClusterBankend(sites siteGetter, networks networkGetter, m modelCluster, hosts hostGetter) *bankendCluster {
	return &bankendCluster{
		sites:    sites,
		networks: networks,
		m:        m,
		hosts:    hosts,
	}
}

type bankendCluster struct {
	sites    siteGetter
	networks networkGetter
	m        modelCluster

	hosts hostGetter
}

type modelCluster interface {
	clusterGetter

	Insert(model.Cluster) (string, error)
	Update(model.Cluster) error
	Delete(name string) error
}

type clusterGetter interface {
	Get(name string) (model.Cluster, error)
	List(selector map[string]string) ([]model.Cluster, error)
}

func (b *bankendCluster) Add(ctx context.Context, config api.ClusterConfig) (api.Cluster, error) {
	site, err := b.sites.Get(config.Site)
	if err != nil {
		return api.Cluster{}, err
	}

	//check name is uniq
	selector := make(map[string]string)
	selector["name"] = config.Name
	sites, err := b.sites.List(selector)
	if err != nil {
		return api.Cluster{}, err
	} else if len(sites) > 0 {
		return api.Cluster{}, stderror.Errorf("cluster name %s already exists", config.Name)
	}

	mc := convertToModelCluster(config, site)
	id, err := b.m.Insert(mc)
	if err != nil {
		return api.Cluster{}, err
	}

	mc.ID = id

	return convertToClusterAPI(mc, nil), err
}

func convertToClusterAPI(mc model.Cluster, networks []model.Network) api.Cluster {
	ids := make([]api.IDName, len(networks))
	for i := range networks {
		ids[i] = api.NewIDName(networks[i].ID, networks[i].Name)
	}

	return api.Cluster{
		ID:       mc.ID,
		Name:     mc.Name,
		Site:     api.NewIDName(mc.SiteID, mc.Site.Name),
		Zone:     mc.Zone,
		HATag:    mc.HaTag,
		Desc:     mc.Desc,
		Enabled:  mc.Enabled,
		Images:   mc.Images.Strings(),
		Networks: ids,
		Created:  api.NewEditor(mc.CreatedUser, mc.CreatedAt),
		Modified: api.NewEditor(mc.ModifiedUser, mc.ModifiedAt),
	}
}

func convertToModelCluster(config api.ClusterConfig, site model.Site) model.Cluster {
	return model.Cluster{
		Name:   config.Name,
		SiteID: site.ID,
		Site: model.SiteBrief{
			ID:   site.ID,
			Name: site.Name,
		},
		Zone:    config.Zone,
		HaTag:   config.HATag,
		Desc:    config.Desc,
		Images:  model.NewSliceString(config.Images),
		Enabled: config.Enabled,
		Editor:  newCreateEditor(config.User),
	}
}

func (b *bankendCluster) List(ctx context.Context, id, name, site, enabled string) ([]api.Cluster, error) {
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
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, nil
	}

	clusters := make([]api.Cluster, len(list))
	selector = make(map[string]string)

	if len(clusters) == 1 {
		selector["cluster_id"] = list[0].ID
	}

	if site != "" {
		selector["site_id"] = site
	}

	networks, err := b.networks.List(selector)

	if err != nil && !model.IsNotExist(err) {
		return clusters, err
	}

	for i := range list {

		out := make([]model.Network, 0, 2)

		for n := range networks {
			if networks[n].ClusterID == list[i].ID {
				out = append(out, networks[n])
			}
		}

		clusters[i] = convertToClusterAPI(list[i], out)
	}

	return clusters, nil
}

func (b *bankendCluster) Set(ctx context.Context, id string, opts api.ClusterOptions) (api.Cluster, error) {
	mc, err := b.m.Get(id)
	if err != nil {
		return api.Cluster{}, err
	}

	mc = mergeCluster(mc, opts)

	err = b.m.Update(mc)

	return convertToClusterAPI(mc, nil), err
}

func mergeCluster(mc model.Cluster, opts api.ClusterOptions) model.Cluster {
	if opts.Name != nil {
		mc.Name = *opts.Name
	}
	if opts.Zone != nil {
		mc.Zone = *opts.Zone
	}
	if opts.HaTag != nil {
		mc.HaTag = *opts.HaTag
	}
	if opts.Images != nil {
		mc.Images = model.NewSliceString(opts.Images)
	}
	if opts.Desc != nil {
		mc.Desc = *opts.Desc
	}
	if opts.Enabled != nil {
		mc.Enabled = *opts.Enabled
	}
	if opts.User != "" {
		mc.ModifiedUser = opts.User
	}

	mc.ModifiedAt = time.Now()

	return mc
}

func (b *bankendCluster) Delete(ctx context.Context, id string) error {
	selector := make(map[string]string)
	selector["cluster_id"] = id

	networkOut, err := b.networks.List(selector)
	if err != nil {
		klog.Errorf("List networks err: %s", err)
		return fmt.Errorf("List networks err: %s", err)
	}

	if networkOut != nil && len(networkOut) != 0 {
		klog.Info("netWork:", networkOut)
		return fmt.Errorf("Please delete network before delete cluster.")
	}

	hostOut, err := b.hosts.List(selector)
	if err != nil {
		klog.Errorf("List hosts err: %s", err)
		return fmt.Errorf("List hosts err: %s", err)
	}
	if hostOut != nil && len(hostOut) != 0 {
		return fmt.Errorf("Please delete hosts before delete cluster.")
	}

	err = b.m.Delete(id)
	klog.Info("Deleting network ...")
	if err == nil || model.IsNotExist(err) {
		return nil
	}

	return err
}
