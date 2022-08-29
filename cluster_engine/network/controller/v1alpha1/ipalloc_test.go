package v1alpha1

import (
	"testing"
)

func TestAllocIP(t *testing.T) {
	mgr := NewNetworkMgr("test")
	mgr.init("192.168.1.17", "192.168.1.30", 24)
	mgr.allocRequest(nil)
	ip, _ := mgr.allocRequest(nil)

	if err := mgr.releaseRequest(ip); err != nil {
		t.Logf("releaseRequest fail%s ", err)
	}

	all, used := mgr.getIPCounts()
	t.Logf("all:%d,used:%d", all, used)
}
