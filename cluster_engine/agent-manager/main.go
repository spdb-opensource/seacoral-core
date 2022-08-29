package main

import (
	"flag"
	"fmt"
	"time"

	clientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	informers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions"
	"github.com/upmio/dbscale-kube/pkg/vars"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	agent "github.com/upmio/dbscale-kube/cluster_engine/agent-manager/controller/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/signals"
)

var (
	masterURL   string
	kubeconfig  string
	hostname    string
	shellDir    string
	versionFlag bool
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&hostname, "hostname", "", "the host name.(can't be empty)")
	flag.StringVar(&shellDir, "shelldir", "/tmp/scripts/", "the shellDir.")
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
}

func main() {
	flag.Parse()
	if versionFlag {
		fmt.Println("the build version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	logs.InitLogs()
	defer logs.FlushLogs()

	if hostname == "" {
		klog.Fatal("the hostname must be set.(which is same as the kubelet hostName)")
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	vpClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building vp clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	vpInformerFactory := informers.NewSharedInformerFactory(vpClient, time.Second*30)
	controller := agent.NewController(kubeClient, vpClient, vpInformerFactory, kubeInformerFactory, shellDir, hostname)

	go kubeInformerFactory.Start(stopCh)
	go vpInformerFactory.Start(stopCh)

	if err = controller.Run(5, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}
