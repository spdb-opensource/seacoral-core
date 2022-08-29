package v1alpha1

import (
	"sync"

	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

var (
	LackResourceErr = xerrors.New("Lack of IP resources")
)

//管理IP接口
type NetworkingMgrInterface interface {
	Init() error

	GetNetwork(network string) (*NetworkMgr, bool)

	AllocRequest(network string, ignores []string) (string, error)
	ReleaseRequest(network, ip string) error

	AddNetwork(network *networkv1.Network) error
	ReleaseNetwork(network *networkv1.Network) error
}

//管理IP
type NetworkingMgr struct {
	name               string
	lock               *sync.Mutex
	networks           map[string]*NetworkMgr
	belongedController *Controller
}

func (networking *NetworkingMgr) GetNetwork(key string) (*NetworkMgr, bool) {
	network, ok := networking.networks[key]
	return network, ok
}

func NewNetworkingMgr(name string, belongedController *Controller) *NetworkingMgr {
	return &NetworkingMgr{
		name:               name,
		networks:           make(map[string]*NetworkMgr),
		belongedController: belongedController,
		lock:               new(sync.Mutex),
	}
}

//初始化，ip资源池初始化
func (networking *NetworkingMgr) Init() error {

	networks, err := networking.belongedController.networkLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, network := range networks {

		key := network.GetName()
		networkMgr := NewNetworkMgr(key)

		if err := networkMgr.init(network.Spec.StartIP, network.Spec.EndIP, network.Spec.Mask); err != nil {
			klog.Errorf("%s netowrk init fail:%s", key, err.Error())
		}

		networking.networks[network.GetName()] = networkMgr
	}

	//ip池初始化
	networkclaims, err := networking.belongedController.networkClaimLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, networkclaim := range networkclaims {

		networkMgr, ok := networking.networks[networkclaim.Spec.Network]
		if !ok {
			klog.Warningf("don't find  %s network by the %s networkclaim ", networkclaim.Spec.Network, networkclaim.GetName())
			continue
		}

		networkMgr.used(networkclaim.Status.BindIP)
	}

	return nil

}

//请求分配IP地址，可并发
func (networking *NetworkingMgr) AllocRequest(network string, ignores []string) (string, error) {
	networkMgr, ok := networking.networks[network]
	if !ok {
		return "", xerrors.Errorf("don't find the network '%s'", network)
	}

	return networkMgr.allocRequest(ignores)
}

//释放IP地址
func (networking *NetworkingMgr) ReleaseRequest(network string, ip string) error {
	networkMgr, ok := networking.networks[network]
	if !ok {
		return xerrors.Errorf("don't find the network '%s'", network)
	}

	return networkMgr.releaseRequest(ip)
}
func (networking *NetworkingMgr) ReleaseNetwork(network *networkv1.Network) error {
	networking.lock.Lock()
	defer networking.lock.Unlock()

	key := network.GetName()
	delete(networking.networks, key)

	return nil
}

func (networking *NetworkingMgr) AddNetwork(network *networkv1.Network) error {
	networking.lock.Lock()
	defer networking.lock.Unlock()

	key := network.GetName()
	networkMgr := NewNetworkMgr(key)

	if err := networkMgr.init(network.Spec.StartIP, network.Spec.EndIP, network.Spec.Mask); err != nil {
		return xerrors.Errorf("%s netowrk init fail:%s", network.GetName(), err.Error())
	}

	networking.networks[key] = networkMgr

	return nil
}

type NetworkMgr struct {
	name      string
	ipPool    map[uint32]bool
	lock      *sync.Mutex
	allocLock *sync.Mutex
}

func NewNetworkMgr(name string) *NetworkMgr {
	return &NetworkMgr{
		name:      name,
		ipPool:    make(map[uint32]bool),
		lock:      new(sync.Mutex),
		allocLock: new(sync.Mutex),
	}
}

func (n *NetworkMgr) getIPCounts() (int32, int32) {
	all := 0
	used := 0

	for _, value := range n.ipPool {
		all += 1
		if value {
			used += 1
		}
	}

	return int32(all), int32(used)
}

func (n *NetworkMgr) init(startIP, endIP string, mask int32) error {
	startU32 := utils.IPToUint32(startIP)
	endU32 := utils.IPToUint32(endIP)

	if move := uint(32 - mask); (startU32 >> move) != (endU32 >> move) {
		return xerrors.Errorf("%s-%s is different network segments", startIP, endIP)
	}
	if startU32 > endU32 {
		startU32, endU32 = endU32, startU32
	}

	num := int(endU32 - startU32 + 1)
	klog.Infof("startip:%ss ,endip;%s, num:%d", startIP, endIP, num)

	for i := 0; i < num; i++ {
		n.ipPool[startU32] = false
		startU32++

	}

	return nil
}

func (n *NetworkMgr) releaseRequest(ip string) error {
	n.allocLock.Lock()
	defer n.allocLock.Unlock()

	if err := n.unUsed(ip); err != nil {
		return err
	}

	return nil
}

/*
func (n *NetworkMgr) pingCheckAndRelease(ip string) (string, error) {
	//ping ip 1s
	return n.releaseRequest(ip)
}

//直到分配成功或者返回资源不足，可并发
func (n *NetworkMgr) allocAndPingCheck() (string, error) {
	for i := 0; i < 200; i++ {
		ip, err := n.allocRequest()
		if err == LackResourceErr {
			return err
		}
		//ping check 1s
	}

}
*/

func (n *NetworkMgr) allocRequest(ignores []string) (string, error) {
	n.allocLock.Lock()
	defer n.allocLock.Unlock()

	for key := range n.ipPool {
		if n.isAlloced(key) {
			continue
		}

		ip := utils.Uint32ToIP(key)

		if utils.ContainsString(ignores, ip) {
			continue
		}

		n.used(ip)

		return ip, nil
	}
	return "", LackResourceErr
}

func (n *NetworkMgr) used(ip string) error {
	if ip == "" {
		return nil
	}
	IPU32 := utils.IPToUint32(ip)

	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.ipPool[IPU32]; !ok {
		return xerrors.Errorf("%s don't in the %s network", ip, n.name)
	}

	n.ipPool[IPU32] = true

	return nil
}

func (n *NetworkMgr) isAlloced(ip uint32) bool {
	if _, ok := n.ipPool[ip]; ok && n.ipPool[ip] {
		return true
	}

	return false
}

func (n *NetworkMgr) unUsed(ip string) error {
	if ip == "" {
		return nil
	}
	IPU32 := utils.IPToUint32(ip)

	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.ipPool[IPU32]; !ok {
		return xerrors.Errorf("%s don't in the %s network", ip, n.name)
	}

	if n.isAlloced(IPU32) {
		n.ipPool[IPU32] = false
	}

	return nil
}

func valicateNetworkSpec() error {
	return nil
}
