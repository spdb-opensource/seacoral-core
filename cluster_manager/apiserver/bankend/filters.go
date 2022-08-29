package bankend

import (
	"fmt"
	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
)

func isInClusters(clusters []string, id string) bool {

	if len(clusters) == 0 {
		return true
	}

	for i := range clusters {

		if clusters[i] == id {
			return true
		}
	}

	return false
}

func (s *scheduler) filterNetworkCondition(cond api.Condition) (map[string][]model.Network, error) {
	var (
		err      error
		networks []model.Network
	)

	if len(cond.Candidates) == 0 {
		networks, err = s.networks.List(map[string]string{})
		if err != nil {
			return nil, err
		}

	} else {
		networks = make([]model.Network, 0, len(cond.Candidates))

		for i := range cond.Candidates {

			nw, err := s.networks.Get(cond.Candidates[i])
			if err == nil {
				networks = append(networks, nw)
			}
		}
	}

	topology := make(map[string][]model.Network)

	for _, net := range networks {

		ss := net.Topology.Strings()

		if len(ss) == 0 {
			topology[net.ID] = []model.Network{net}
			continue
		}

		for _, topo := range ss {

			list, ok := topology[topo]
			if !ok {
				topology[topo] = []model.Network{net}
			} else {

				list = append(list, net)
				topology[topo] = list
			}
		}
	}

	if cond.HighAvailability {

		for key, list := range topology {
			if len(list) < 2 {
				delete(topology, key)
			}
		}
	}

	if len(topology) == 0 {
		return nil, fmt.Errorf("non network matched on condition: %v", cond)
	}

	return topology, nil
}

func (s *scheduler) filterClusterCondition(cond api.Condition, image string) ([]model.Cluster, error) {
	var (
		err  error
		list []model.Cluster
	)

	if len(cond.Candidates) == 0 {

		list, err = s.clusters.List(map[string]string{})
		if err != nil {
			return nil, err
		}
	} else {

		list = make([]model.Cluster, 0, len(cond.Candidates))

		for i := range cond.Candidates {

			cluster, err := s.clusters.Get(cond.Candidates[i])
			if err == nil {
				list = append(list, cluster)
			}
		}
	}

	out := make([]model.Cluster, 0, len(list))
	for i := range list {
		if list[i].Images.In(image) {
			out = append(out, list[i])
		}
	}

	if len(out) == 0 || (cond.HighAvailability && len(list) < 2) {
		return nil, fmt.Errorf("non cluster matched on image %s and condition: %v,got %d", image, cond, len(list))
	}

	return out, nil
}

func (s *scheduler) filterHostCondition(replicas int, cond api.Condition) (map[string][]model.Host, error) {

	var (
		err   error
		hosts []model.Host
	)

	if len(cond.Candidates) == 0 {

		hosts, err = s.hosts.List(map[string]string{})
		if err != nil {
			return nil, err
		}

	} else {

		hosts = make([]model.Host, 0, len(cond.Candidates))

		for i := range cond.Candidates {

			host, err := s.hosts.Get(cond.Candidates[i])
			if err == nil {
				hosts = append(hosts, host)
			}
		}
	}
	klog.Info(hosts)

	if cond.HighAvailability && len(hosts) < replicas {
		return nil, fmt.Errorf("no enough hosts %d < %d on condition : %v", len(hosts), replicas, cond)
	}

	out := make(map[string][]model.Host, len(hosts))

	for _, host := range hosts {

		list, ok := out[host.ClusterID]
		if !ok {

			out[host.ClusterID] = []model.Host{host}

		} else {

			list = append(list, host)
			out[host.ClusterID] = list
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("non host matched on condition :%v", cond)
	}

	return out, nil
}

func (s *scheduler) filterRemoteStorageCondition(requests api.StorageRequirement, cond api.Condition) ([]model.RemoteStorage, error) {

	if requests.Type == api.StorageTypeHost {
		return nil, nil
	}

	var (
		err  error
		list []model.RemoteStorage
	)

	if len(cond.Candidates) == 0 {
		list, err = s.storages.List(map[string]string{})
		if err != nil {
			return nil, err
		}

	} else {

		list = make([]model.RemoteStorage, 0, len(cond.Candidates))

		for i := range cond.Candidates {

			rs, err := s.storages.Get(cond.Candidates[i])
			if err == nil {
				list = append(list, rs)
			}
		}
	}

	if requests.Performance != "" {
		out := make([]model.RemoteStorage, 0, len(list))

	loop:
		for i := range list {

			pools, _ := s.pools.ListPools(map[string]string{
				"remote_storage_id": list[i].ID,
			})

			for k := range pools {
				if pools[k].Performance == string(requests.Performance) {

					out = append(out, list[i])
					continue loop
				}
			}
		}

		list = out
	}

	if len(list) == 0 || (cond.HighAvailability && len(list) < 2) {
		return nil, fmt.Errorf("no enough RemoteStorages matched on condition: %v,got %d", cond, len(list))
	}

	return list, nil
}
