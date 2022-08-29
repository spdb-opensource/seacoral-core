package bankend

const (
	labelEnabled       = "enabled"
	labelUnschedulable = "unschedulable"
	labelCluster       = "dbscale.cluster"
	labelSite          = "dbscale.site"
	labelRemoteStorage = "dbscale.storage.remote"
	labelDbscaleIo     = "dbscale.io"
	labelRole          = "kubernetes.io/role"

	labelAppID                = "dbscale.app.id"
	labelGroupName            = "dbscale.app.group"
	labelGroupType            = "dbscale.service.group.type"
	labelAppIDAndGroup        = "dbscale.app.id.and.group.type"
	labelServiceType          = "dbscale.service.type"
	labelGroupHostHA          = "ha.host.group"
	labelServiceName          = "dbscale.service.name"
	labelServiceImageType     = "dbscale.service.image.name"
	labelNfsClientProvisioner = "nfs-client-provisioner"
	// lijj32: add a new label which presents the allocatable resource level of a node
	labelResourceAllocatable = "resource.allocatable.level"

	annotationVGRequest      = "vg.localvolume.request"
	annotationHostUsageLimit = "node.usage.limit"
	annotationHostSpare      = "upm.host.spare"

	labelRelease        = "release"
	PrometheusNameSpace = "prometheus"
	KubeSystemNameSpace = "kube-system"
)
