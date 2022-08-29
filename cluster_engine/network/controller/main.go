package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	clientset "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
	informers "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/informers/externalversions"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	controller "github.com/upmio/dbscale-kube/cluster_engine/network/controller/v1alpha1"
	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking"
	"github.com/upmio/dbscale-kube/pkg/signals"
	"github.com/upmio/dbscale-kube/pkg/vars"
	v1beta1apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

var (
	versionFlag bool

	masterURL  string
	kubeconfig string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
}

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("the build version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	err = initCRDs(cfg)
	if err != nil {
		klog.Errorf("Error init crds: %s", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	networkingClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	networkingInformerFactory := informers.NewSharedInformerFactory(networkingClient, time.Second*30)

	controller := controller.NewController(kubeClient, networkingClient, kubeInformerFactory, networkingInformerFactory)

	go kubeInformerFactory.Start(stopCh)
	go networkingInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func initCRDs(config *rest.Config) error {
	client, err := apiextensions.NewForConfig(config)
	if err != nil {
		return err
	}

	errs := make([]string, 0, 2)
	_, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), &v1beta1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "networks." + networkv1.GroupName,
		},
		Spec: v1beta1apiextensions.CustomResourceDefinitionSpec{
			Group: networkv1.GroupName,
			Names: v1beta1apiextensions.CustomResourceDefinitionNames{
				Kind:     "Network",
				ListKind: "NetworkList",
				Plural:   "networks",
			},
			Scope:   v1beta1apiextensions.ClusterScoped,
			Version: networkv1.Version,
			Subresources: &v1beta1apiextensions.CustomResourceSubresources{
				Status: &v1beta1apiextensions.CustomResourceSubresourceStatus{},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err.Error())
	}

	_, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), &v1beta1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "networkclaims." + networkv1.GroupName,
		},
		Spec: v1beta1apiextensions.CustomResourceDefinitionSpec{
			Group: networkv1.GroupName,
			Names: v1beta1apiextensions.CustomResourceDefinitionNames{
				Kind:     "NetworkClaim",
				ListKind: "NetworkClaimList",
				Plural:   "networkclaims",
			},
			Scope:   v1beta1apiextensions.ClusterScoped,
			Version: networkv1.Version,
			Subresources: &v1beta1apiextensions.CustomResourceSubresources{
				Status: &v1beta1apiextensions.CustomResourceSubresourceStatus{},
			},
		},
	}, metav1.CreateOptions{})

	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err.Error())
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "\n"))
}
