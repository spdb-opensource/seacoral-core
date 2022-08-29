package bankend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/zone"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func NewNetworkBankend(zone zone.ZoneInterface, m modelNetwork, sites siteGetter, clusters clusterGetter) *bankendNetwork {
	return &bankendNetwork{
		m:        m,
		zone:     zone,
		sites:    sites,
		clusters: clusters,
	}
}

type bankendNetwork struct {
	m        modelNetwork
	clusters clusterGetter
	sites    siteGetter
	zone     zone.ZoneInterface
}

type modelNetwork interface {
	Insert(model.Network) (string, error)
	Update(model.Network) error
	Delete(name string) error

	networkGetter
}

type networkGetter interface {
	Get(name string) (model.Network, error)
	List(selector map[string]string) ([]model.Network, error)
}

func (b *bankendNetwork) Add(ctx context.Context, config api.NetworkConfig) (api.Network, error) {
	var err error
	site := model.Site{}
	cluster := model.Cluster{}

	if config.Cluster != "" {

		cluster, err = b.clusters.Get(config.Cluster)
		if err != nil {
			return api.Network{}, err
		}
		config.Cluster = cluster.ID
		config.Site = cluster.SiteID
		site.ID = cluster.SiteID

	}

	if config.Site != "" {
		site, err = b.sites.Get(config.Site)
		if err != nil {
			return api.Network{}, err
		}

		config.Site = site.ID
	} else {
		site, err = b.sites.Get(cluster.SiteID)
		if err != nil {
			return api.Network{}, err
		}
	}

	iface, err := b.zone.NetworkInterface(config.Site)
	if err != nil {
		return api.Network{}, err
	}

	net := convertToModelNetwork(config, cluster)
	networking := convertToNetworkingV1alpha1(config)

	networking.Spec.Mode = v1alpha1.NetworkModeType(site.NetworkMode)
	networking.Name = net.ObjectName()
	setNetworkingLables(networking, cluster)

	networking, err = iface.Create(networking)
	if err != nil {
		return api.Network{}, err
	}

	id, err := b.m.Insert(net)
	if err == nil {
		net.ID = id
	}

	return convertToNetworkAPI(net, *networking), err
}

func setNetworkingLables(net *v1alpha1.Network, c model.Cluster) {
	if net.Labels == nil {
		net.Labels = make(map[string]string)
	}

	net.Labels[labelCluster] = c.ID
	net.Labels[labelSite] = c.SiteID
}

func convertToNetworkingV1alpha1(config api.NetworkConfig) *v1alpha1.Network {
	return &v1alpha1.Network{
		Spec: v1alpha1.NetworkSpec{
			//Mode:    v1alpha1.NetworkModeType(config.NetworkMode),
			StartIP: string(config.IPSummary.Start),
			EndIP:   string(config.IPSummary.End),
			Mask:    config.IPSummary.Prefix,
			Route:   string(config.IPSummary.Gateway),
			Vlan:    config.IPSummary.VLan,
		},
	}
}

func convertToModelNetwork(config api.NetworkConfig, cluster model.Cluster) model.Network {
	end := config.IPSummary.End.Parse()
	name := fmt.Sprintf("%s-%d", config.IPSummary.Start.String(), end[len(end)-1])

	return model.Network{
		ID:        utils.NewUUID(),
		Name:      name,
		ClusterID: cluster.ID,
		Cluster: model.ClusterBrief{
			ID:     cluster.ID,
			Name:   cluster.Name,
			SiteID: cluster.SiteID,
		},
		Desc:     config.Desc,
		Topology: model.NewSliceString(config.Topology),
		Enabled:  config.Enabled,
		Editor:   newCreateEditor(config.User),
	}
}

func convertToNetworkAPI(mn model.Network, net v1alpha1.Network) api.Network {
	return api.Network{
		ID:       mn.ID,
		Name:     mn.Name,
		Site:     api.NewIDName(mn.Site.ID, mn.Site.Name),
		Cluster:  api.NewIDName(mn.ClusterID, mn.Cluster.Name),
		Desc:     mn.Desc,
		Enabled:  mn.Enabled,
		Topology: mn.Topology.Strings(),
		IP: api.IPSummary{
			Total: int(net.Status.AllIPCounts),
			Used:  int(net.Status.UsedIPCount),
			Route: api.Route{
				Prefix:  net.Spec.Mask,
				VLan:    net.Spec.Vlan,
				Start:   api.IP(net.Spec.StartIP),
				End:     api.IP(net.Spec.EndIP),
				Gateway: api.IP(net.Spec.Route),
			},
		},
		Created:  api.NewEditor(mn.CreatedUser, mn.CreatedAt),
		Modified: api.NewEditor(mn.ModifiedUser, mn.ModifiedAt),
	}
}

func (b *bankendNetwork) List(ctx context.Context, id, name, cluster, site, topology, enabled string) ([]api.Network, error) {
	selector := make(map[string]string)
	if topology != "" {
	} else {
		if id != "" {
			selector["id"] = id
		}
		if name != "" {
			selector["name"] = name
		}
		if cluster != "" {
			selector["cluster_id"] = cluster
		}
		if site != "" {
			selector["site_id"] = site
		}
		if enabled != "" {
			selector[labelEnabled] = boolValue(enabled)
		}
	}

	list, err := b.m.List(selector)
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if topology != "" {
		out := make([]model.Network, 0, len(list))
	loop:
		for i := range list {
			if !strings.Contains(string(list[i].Topology), topology) {
				continue
			}

			ss := list[i].Topology.Strings()
			for j := range ss {

				if ss[j] == topology {

					out = append(out, list[i])

					continue loop
				}
			}
		}

		list = out
	}

	errs := make([]error, 0, len(list))
	nm := make(map[string]v1alpha1.Network)
	networks := make([]api.Network, len(list))

	for i := range list {

		if networking, ok := nm[list[i].ID]; ok {

			networks[i] = convertToNetworkAPI(list[i], networking)
			continue
		}

		iface, err := b.zone.NetworkInterface(list[i].Cluster.SiteID)
		if err != nil {

			errs = append(errs, err)
			networks[i] = convertToNetworkAPI(list[i], v1alpha1.Network{})
			continue
		}

		networking, err := iface.Get(list[i].ObjectName())
		if err != nil {

			errs = append(errs, err)
			networks[i] = convertToNetworkAPI(list[i], v1alpha1.Network{})
			continue
		}

		nm[list[i].ID] = *networking

		networks[i] = convertToNetworkAPI(list[i], *networking)
	}

	if err := utilerrors.NewAggregate(errs); err != nil {
		klog.Errorln("network list:", err)
	}

	return networks, nil
}

func (b *bankendNetwork) Set(ctx context.Context, id string, opts api.NetworkOptions) (api.Network, error) {
	net := api.Network{}
	mn, err := b.m.Get(id)
	if err != nil {
		return net, err
	}

	if opts.Cluster != nil {
		cluster, err := b.clusters.Get(*opts.Cluster)
		if err != nil {
			return net, err
		}

		mn.ClusterID = cluster.ID
		mn.Cluster.Name = cluster.Name
		mn.Cluster.SiteID = cluster.SiteID
	}

	mn = mergeNetwork(mn, opts)

	err = b.m.Update(mn)

	return convertToNetworkAPI(mn, v1alpha1.Network{}), err
}

func mergeNetwork(mn model.Network, opts api.NetworkOptions) model.Network {

	if opts.Desc != nil {
		mn.Desc = *opts.Desc
	}
	if opts.Topology != nil {
		mn.Topology = model.NewSliceString(opts.Topology)
	}
	if opts.Enabled != nil {
		mn.Enabled = *opts.Enabled
	}
	if opts.User != "" {
		mn.ModifiedUser = opts.User
	}

	mn.ModifiedAt = time.Now()

	return mn
}

func (b *bankendNetwork) Delete(ctx context.Context, id string) error {
	mn, err := b.m.Get(id)
	if model.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	iface, err := b.zone.NetworkInterface(mn.Cluster.SiteID)
	if err != nil {
		return err
	}

	err = iface.Delete(mn.ObjectName(), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return b.m.Delete(mn.ID)
}
