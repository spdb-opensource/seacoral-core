package main

import (
	"context"
	"flag"
	"fmt"
	execserver "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice"
	"math/rand"
	"os"
	"time"

	nwclientset "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
	nwinformers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/informers/externalversions"
	sanclientset "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	saninformers "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/informers/externalversions"
	unitclientset "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/clientset/versioned"
	unitinformers "github.com/upmio/dbscale-kube/pkg/client/unit/v1alpha4/informers/externalversions"
	lvmclientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	lvminformers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions"
	"github.com/upmio/dbscale-kube/pkg/signals"
	"github.com/upmio/dbscale-kube/pkg/vars"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	hostclientset "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
	hostInformers "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/informers/externalversions"
)

const controllerManagerUserAgent = "dbscale-kube-controller-manager"

var (
	versionFlag bool
	execServer  string
	masterURL   string
	kubeconfig  string
	script      = "/opt/kube/scripts/StorMGR/StorMGR"

	LeaderElection = &componentbaseconfig.LeaderElectionConfiguration{
		LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
		RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
		RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
		ResourceLock:  resourcelock.EndpointsLeasesResourceLock,
		LeaderElect:   true,
	}
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
	flag.StringVar(&script, "scripts", script, "path to storage script dir.")
	flag.StringVar(&execServer, "exec-server", execServer, "addr of exec service")

	flag.BoolVar(&LeaderElection.LeaderElect, "leader-elect", LeaderElection.LeaderElect, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	flag.DurationVar(&LeaderElection.LeaseDuration.Duration, "leader-elect-lease-duration", LeaderElection.LeaseDuration.Duration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	flag.DurationVar(&LeaderElection.RenewDeadline.Duration, "leader-elect-renew-deadline", LeaderElection.RenewDeadline.Duration, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	flag.DurationVar(&LeaderElection.RetryPeriod.Duration, "leader-elect-retry-period", LeaderElection.RetryPeriod.Duration, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	flag.StringVar(&LeaderElection.ResourceLock, "leader-elect-resource-lock", LeaderElection.ResourceLock, ""+
		"The type of resource object that is used for locking during "+
		"leader election. Supported options are `endpoints` (default) and `configmaps`.")
}

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	rand.Seed(time.Now().UTC().UnixNano())
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Info("VERSION: ", vars.GITCOMMIT, " ", vars.BUILDTIME, fmt.Sprintf("  '%s'", vars.SeCretAESKey))

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err)
	}

	config = restclient.AddUserAgent(config, controllerManagerUserAgent)

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err)
	}

	err = initCRDs(config)
	if err != nil {
		klog.Fatalf("Error init CRDs: %s", err)
	}

	err = initCSIDriver(kubeClient)
	if err != nil {
		klog.Fatalf("Error init vp csidriver: %s", err)
	}

	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-stopCh
		cancel()
	}()

	go func() {
		klog.Infof("%s:stsrt ExecServer", execServer)
		clients := &connects{
			key:    vars.SeCretAESKey,
			script: script,
		}
		err = clients.init(config, 30*time.Second)
		if err != nil {
			klog.Fatalf("Error init connects: %s", err)
		}
		execserver.SetupExecServer(execServer, clients.config, nil, ctx.Done())
	}()
	run := func(ctx context.Context) {
		clients := &connects{
			key:    vars.SeCretAESKey,
			script: script,
		}

		err = clients.init(config, 30*time.Second)
		if err != nil {
			klog.Fatalf("Error init connects: %s", err)
		}

		controllers := knownControllers(clients, true, true, true)

		clients.Start(controllers, ctx.Done())

		//go execserver.SetupExecServer(execServer, clients.config, nil, ctx.Done())
	}

	var electionChecker *leaderelection.HealthzAdaptor

	if LeaderElection.LeaderElect {

		electionChecker = leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)

	} else {
		run(ctx)

		<-stopCh

		os.Exit(1)
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Error: %s", err)
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(resourcelock.LeasesResourceLock,
		metav1.NamespaceDefault,
		controllerManagerUserAgent,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: createRecorder(kubeClient, controllerManagerUserAgent),
		})
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: LeaderElection.LeaseDuration.Duration,
		RenewDeadline: LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leader election lost")
			},
		},
		WatchDog: electionChecker,
		Name:     controllerManagerUserAgent,
	})

	<-stopCh
	klog.Fatal("Never running here !!!")
}

type connects struct {
	key    string
	script string

	config *restclient.Config

	kubeClient          kubernetes.Interface
	kubeInformerFactory kubeinformers.SharedInformerFactory

	networkClient          nwclientset.Interface
	networkInformerFactory nwinformers.SharedInformerFactory

	sanClient          sanclientset.Interface
	sanInformerFactory saninformers.SharedInformerFactory

	unitClient          unitclientset.Interface
	unitInformerFactory unitinformers.SharedInformerFactory

	lvmClient   lvmclientset.Interface
	lvminformer lvminformers.SharedInformerFactory

	hostClient          hostclientset.Interface
	hostInformerFactory hostInformers.SharedInformerFactory
}

func (ctx *connects) init(config *restclient.Config, defaultResync time.Duration) (err error) {
	ctx.config = config
	ctx.kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building kubernetes clientset: %s", err)
	}

	ctx.networkClient, err = nwclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building networking clientset: %s", err)
	}

	ctx.sanClient, err = sanclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building storage clientset: %s", err)
	}

	ctx.unitClient, err = unitclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building unit clientset: %s", err)
	}

	ctx.lvmClient, err = lvmclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building lvm clientset: %s", err)
	}

	ctx.hostClient, err = hostclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error building host clientset: %s", err)
	}

	ctx.kubeInformerFactory = kubeinformers.NewSharedInformerFactory(ctx.kubeClient, defaultResync)
	ctx.networkInformerFactory = nwinformers.NewSharedInformerFactory(ctx.networkClient, defaultResync)
	ctx.sanInformerFactory = saninformers.NewSharedInformerFactory(ctx.sanClient, defaultResync)
	ctx.lvminformer = lvminformers.NewSharedInformerFactory(ctx.lvmClient, defaultResync)
	ctx.unitInformerFactory = unitinformers.NewSharedInformerFactory(ctx.unitClient, defaultResync)
	ctx.hostInformerFactory = hostInformers.NewSharedInformerFactory(ctx.hostClient, defaultResync)

	return nil
}

func (ctx connects) Start(controllers []controller, stopCh <-chan struct{}) {

	ctx.kubeInformerFactory.Start(stopCh)
	ctx.networkInformerFactory.Start(stopCh)
	ctx.sanInformerFactory.Start(stopCh)
	ctx.unitInformerFactory.Start(stopCh)
	ctx.lvminformer.Start(stopCh)
	ctx.hostInformerFactory.Start(stopCh)

	for i := range controllers {
		ctr := controllers[i]
		go func() {
			err := ctr.Run(15, stopCh)
			if err != nil {
				klog.Fatalf("should not happen:%s", err.Error())
			}
		}()
	}
}

func createRecorder(kubeClient kubernetes.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	// TODO: remove dependency on the legacyscheme
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: userAgent})
}
