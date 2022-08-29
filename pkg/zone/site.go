package zone

import (
	"fmt"
	"sync"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	serviceMonitor "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"

	hostInformers "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/informers/externalversions"
	nwinformers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/informers/externalversions"
	networklisters "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/listers/networking/v1alpha1"
	saninformers "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/informers/externalversions"
	unitinformers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/informers/externalversions"
	listers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/listers/unit/v1alpha4"
	lvminformers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions"
	vplisters "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/listers/volumepath/v1alpha1"

	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	host "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
	network "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
	san "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	unitv4 "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/clientset/versioned"
	lvm "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	"github.com/upmio/dbscale-kube/pkg/zone/site"
)

//const defaultConfigDir = "/opt/kube/config"

type k8sSite struct {
	name       string
	execAddr   string
	masterURL  string
	configPath string

	site        site.Interface
	cachelister *CacheLister
	factory     *InformerFactory
	config      *restclient.Config

	stopCh        chan struct{}
	defaultResync time.Duration
	once          *sync.Once
}

type InformerFactory struct {
	kubeFactory            kubeinformers.SharedInformerFactory
	unitInformerFactory    unitinformers.SharedInformerFactory
	networkInformerFactory nwinformers.SharedInformerFactory

	lvminformerFactory  lvminformers.SharedInformerFactory
	sanInformerFactory  saninformers.SharedInformerFactory
	hostInformerFactory hostInformers.SharedInformerFactory
}

type CacheLister struct {
	UnitCache  listers.UnitLister
	unitSynced cache.InformerSynced

	PodCache  corelisters.PodLister
	podSynced cache.InformerSynced

	NetworkClaimCache  networklisters.NetworkClaimLister
	networkClaimSynced cache.InformerSynced

	NodeCache  corelisters.NodeLister
	nodeSynced cache.InformerSynced

	VolumePathCache  vplisters.VolumePathLister
	volumePathSynced cache.InformerSynced
	//PvcCache corelisters.PersistentVolumeClaimLister
	//PvcSynced cache.InformerSynced

	//PvCache corelisters.PersistentVolumeLister
	//PvSynced cache.InformerSynced

	//HostCache  hostlisters.HostLister
	//hostSynced cache.InformerSynced
}

func NewK8sSite(name, domain, execAddr, path string, port int) *k8sSite {
	master := fmt.Sprintf("https://%s:%d", domain, port)

	return &k8sSite{
		name:       name,
		execAddr:   execAddr,
		masterURL:  master,
		configPath: path,
		stopCh:     make(chan struct{}),
		once:       &sync.Once{},
	}
}

func (ks *k8sSite) Close() {
	if ks.stopCh != nil {
		close(ks.stopCh)
		ks.stopCh = nil
	}
}

func (ks *k8sSite) initSite() error {
	if len(ks.masterURL) == 0 {
		return xerrors.New("Site without master")
	}

	err := ks.createSiteInterface(ks.masterURL, ks.configPath)
	if err != nil {
		return err
	}

	err = ks.WaitSyncForCache()
	if err != nil {
		return err
	}
	return nil
}

func (ks *k8sSite) WaitSyncForCache() error {
	if ks.cachelister == nil {
		return fmt.Errorf("%s:cachelister   is nil", ks.name)
	}

	if !cache.WaitForCacheSync(ks.stopCh, ks.cachelister.networkClaimSynced, ks.cachelister.unitSynced, ks.cachelister.podSynced,
		ks.cachelister.volumePathSynced, ks.cachelister.nodeSynced) {
		return fmt.Errorf("%s:Unable to sync caches", ks.name)
	}

	return nil
}

func (ks *k8sSite) createSiteInterface(url, path string) error {
	config, err := clientcmd.BuildConfigFromFlags(url, path)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = kubeClient.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	netClient, err := network.NewForConfig(config)
	if err != nil {
		return err
	}

	sanClient, err := san.NewForConfig(config)
	if err != nil {
		return err
	}

	hostClient, err := host.NewForConfig(config)
	if err != nil {
		return err
	}

	lvmClient, err := lvm.NewForConfig(config)
	if err != nil {
		return err
	}

	unitClient, err := unitv4.NewForConfig(config)
	if err != nil {
		return err
	}

	monitorV1Client, err := serviceMonitor.NewForConfig(config)
	if err != nil {
		return err
	}

	ks.config = config
	//sc := ks.site

	if ks.site == nil {
		ks.site = site.NewClientset(ks.execAddr, kubeClient, netClient, sanClient, hostClient, lvmClient, unitClient, monitorV1Client, nil, ks.defaultResync, ks.stopCh)
	}

	if ks.cachelister == nil {
		if !ks.IsHealthy() {
			return xerrors.Errorf("%s is not health;not to do sync cachelister", ks.name)
		}
		ks.once.Do(
			func() {
				defaultResync := 30 * time.Second
				kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, defaultResync)
				unitInformerFactory := unitinformers.NewSharedInformerFactory(unitClient, defaultResync)
				networkInformerFactory := nwinformers.NewSharedInformerFactory(netClient, defaultResync)
				sanInformerFactory := saninformers.NewSharedInformerFactory(sanClient, defaultResync)
				hostInformerFactory := hostInformers.NewSharedInformerFactory(hostClient, defaultResync)
				lvminformerFactory := lvminformers.NewSharedInformerFactory(lvmClient, defaultResync)

				ks.factory = &InformerFactory{
					kubeFactory:            kubeInformerFactory,
					networkInformerFactory: networkInformerFactory,
					unitInformerFactory:    unitInformerFactory,
					lvminformerFactory:     lvminformerFactory,
					hostInformerFactory:    hostInformerFactory,
					sanInformerFactory:     sanInformerFactory,
				}

				ks.cachelister = &CacheLister{
					podSynced: kubeInformerFactory.Core().V1().Pods().Informer().HasSynced,
					PodCache:  kubeInformerFactory.Core().V1().Pods().Lister(),

					UnitCache:  unitInformerFactory.Unit().V1alpha4().Units().Lister(),
					unitSynced: unitInformerFactory.Unit().V1alpha4().Units().Informer().HasSynced,

					NetworkClaimCache:  networkInformerFactory.Networking().V1alpha1().NetworkClaims().Lister(),
					networkClaimSynced: networkInformerFactory.Networking().V1alpha1().NetworkClaims().Informer().HasSynced,

					NodeCache:  kubeInformerFactory.Core().V1().Nodes().Lister(),
					nodeSynced: kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,

					volumePathSynced: lvminformerFactory.Lvm().V1alpha1().VolumePaths().Informer().HasSynced,
					VolumePathCache:  lvminformerFactory.Lvm().V1alpha1().VolumePaths().Lister(),
				}

				kubeInformerFactory.Start(ks.stopCh)
				unitInformerFactory.Start(ks.stopCh)
				networkInformerFactory.Start(ks.stopCh)
				lvminformerFactory.Start(ks.stopCh)
			})
	}

	//if sc != nil {
	//	for key, fn := range sc.Controllers() {
	//		ks.site.RegisterController(key, fn)
	//	}
	//}

	return nil
}

//func (ks *k8sSite) HandleError(err error) {
//	if err == nil ||
//		!net.IsProbableEOF(err) ||
//		ks.IsHealthy() {
//
//		return
//	}
//
//	ks.initSite()
//}

func (ks k8sSite) Name() string {
	return ks.name
}

func (ks k8sSite) IsHealthy() bool {
	if ks.site == nil {
		return false
	}

	err := ks.site.Connected()
	if err == nil {
		return true
	}

	return false
}

func (ks k8sSite) Config() (*restclient.Config, error) {
	if ks.config != nil {
		return ks.config, nil
	}
	//
	//err := ks.initSite()
	//if err == nil {
	//	return ks.config, nil
	//}

	return nil, fmt.Errorf("%s config is nil", ks.name)
}

func (ks k8sSite) SiteInterface() (site.Interface, error) {
	if ks.site != nil {
		return ks.site, nil
	}

	err := ks.initSite()
	if err == nil {
		return ks.site, nil
	}

	return nil, err
}

func (ks k8sSite) CacheLister() (*CacheLister, error) {
	if ks.cachelister != nil {
		return ks.cachelister, nil
	}

	err := ks.createSiteInterface(ks.masterURL, ks.configPath)
	if err != nil {
		return nil, err
	}

	err = ks.WaitSyncForCache()
	if err != nil {
		return nil, err
	}

	return ks.cachelister, nil

}
