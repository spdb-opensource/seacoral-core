package main

import (
	hostctrl "github.com/upmio/dbscale-kube/cluster_engine/host/controller/v1alpha1"
	// imagectrl "github.com/upmio/dbscale-kube/cluster_engine/image/controller/v1alpha1"
	networkctrl "github.com/upmio/dbscale-kube/cluster_engine/network/controller/v1alpha1"
	sanctrl "github.com/upmio/dbscale-kube/cluster_engine/storage/controller/v1alpha1"
	unitctrl "github.com/upmio/dbscale-kube/cluster_engine/unit/v1alpha4"
)

type controller interface {
	Run(threadiness int, stopCh <-chan struct{}) error
}

func knownControllers(ctx *connects, unit, san, network bool) []controller {
	controllers := make([]controller, 0, 3)

	if san {
		ctrl := sanctrl.NewController(
			ctx.key, ctx.script,
			ctx.kubeClient, ctx.sanClient, ctx.hostClient, ctx.lvmClient,
			ctx.kubeInformerFactory.Core().V1(),
			ctx.sanInformerFactory.San().V1alpha1(),
			ctx.hostInformerFactory.Host().V1alpha1().Hosts(),
			ctx.lvminformer.Lvm().V1alpha1().VolumePaths())

		controllers = append(controllers, ctrl)
	}

	if unit {
		ctrl := unitctrl.NewController(
			ctx.config,
			ctx.kubeClient,
			ctx.unitClient,
			ctx.networkClient,
			ctx.sanClient,
			ctx.lvmClient,
			ctx.kubeInformerFactory.Core().V1(),
			ctx.networkInformerFactory.Networking().V1alpha1(),
			ctx.unitInformerFactory.Unit().V1alpha4(),
		)

		controllers = append(controllers, ctrl)
	}

	if network {
		ctrl := networkctrl.NewController(
			ctx.kubeClient,
			ctx.networkClient,
			ctx.kubeInformerFactory,
			ctx.networkInformerFactory)

		controllers = append(controllers, ctrl)
	}

	controllers = append(controllers, hostctrl.NewController(
		ctx.kubeClient,
		ctx.hostClient,
		ctx.kubeInformerFactory,
		ctx.hostInformerFactory,
		ctx.unitInformerFactory))

	// controllers = append(controllers, imagectrl.NewController(
	// 	ctx.kubeClient,
	// 	ctx.unitClient,
	// 	ctx.unitInformerFactory.Unit().V1alpha4(),
	// ))

	return controllers
}
