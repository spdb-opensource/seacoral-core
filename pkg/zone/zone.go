package zone

import (
	"sync"

	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/pkg/zone/site"
	"golang.org/x/xerrors"
	restclient "k8s.io/client-go/rest"
)

type ZoneInterface interface {
	GetSite(name string) (Site, error)
	RemoveSite(name string) error

	AddSite(c Site) error
	ListSites() []Site

	SiteInterface
}

type SiteInterface interface {
	SiteInterface(site string) (site.Interface, error)

	HostInterface(site string) (site.HostInterface, error)

	SanSystemInterface(site string) (site.SanSystemInterface, error)

	NetworkInterface(site string) (site.NetworkInterface, error)
	NetworkClaimInterface(site string) (site.NetworkClaimInterface, error)

	UnitInterface(site string) (site.UnitInterface, error)

	CacheLister(site string) (*CacheLister, error)
}

type Site interface {
	Name() string
	IsHealthy() bool
	initSite() error
	Close()
	Config() (*restclient.Config, error)
	SiteInterface() (site.Interface, error)
	CacheLister() (*CacheLister, error)
}

var _ ZoneInterface = &zone{}

type zone struct {
	lock  *sync.RWMutex
	sites map[string]Site
}

func NewZone(n int) *zone {
	return &zone{
		lock:  new(sync.RWMutex),
		sites: make(map[string]Site, n),
	}
}

func (z *zone) GetSite(name string) (Site, error) {
	z.lock.RLock()
	defer z.lock.RUnlock()
	c := z.getSiteByKey(name)

	if c == nil {
		return nil, stderror.Errorf("GetSite is null by %s", name)
	}

	return c, nil
}

func (z *zone) RemoveSite(name string) error {
	z.lock.RLock()
	defer z.lock.RUnlock()

	c := z.getSiteByKey(name)
	if c != nil {
		c.Close()
	}

	delete(z.sites, name)
	return nil
}

func (z *zone) getSiteByKey(key string) Site {
	site, ok := z.sites[key]
	if !ok {
		return nil
	}

	return site
}

func (z *zone) AddSite(c Site) error {
	z.lock.Lock()
	z.lock.Unlock()

	err := z.addSite(c)
	return err
}

func (z *zone) addSite(c Site) error {
	if err := c.initSite(); err != nil {
		return err
	}

	if !c.IsHealthy() {
		return xerrors.New("Site is unhealthy")
	}

	z.sites[c.Name()] = c

	return nil
}

func (z *zone) UpdateSite(c Site) error {
	z.lock.Lock()
	err := z.addSite(c)
	z.lock.Unlock()

	return err
}

func (z *zone) ListSites() []Site {
	z.lock.RLock()

	list := make([]Site, 0, len(z.sites))

	for _, c := range z.sites {
		list = append(list, c)
	}

	z.lock.RUnlock()

	return list
}

func (z *zone) SiteInterface(site string) (site.Interface, error) {
	st, err := z.GetSite(site)
	if err != nil {
		return nil, err
	}

	return st.SiteInterface()
}

func (z *zone) CacheLister(site string) (*CacheLister, error) {
	st, err := z.GetSite(site)
	if err != nil {
		return nil, err
	}

	return st.CacheLister()
}

func (z *zone) HostInterface(site string) (site.HostInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, stderror.Errorf("site: %s get SiteInterface err: %s", site, err)
	}

	return iface.Hosts(), nil
}

func (z *zone) SanSystemInterface(site string) (site.SanSystemInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.SanSystems(), nil
}

func (z *zone) NetworkInterface(site string) (site.NetworkInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.Networks(), nil
}

func (z *zone) NetworkClaimInterface(site string) (site.NetworkClaimInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.NetworkClaims(), nil
}

func (z *zone) UnitInterface(site string) (site.UnitInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.Units(), nil
}

func (z *zone) PodExec(site string) (site.PodExecInterface, error) {
	iface, err := z.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.PodExec(), nil
}
