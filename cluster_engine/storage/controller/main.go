package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/upmio/dbscale-kube/pkg/signals"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	sanctrlv1 "github.com/upmio/dbscale-kube/cluster_engine/storage/controller/v1alpha1"
	sanv1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	lvmv1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	hostclientset "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/clientset/versioned"
	hostinformers "github.com/upmio/dbscale-kube/pkg/client/host/v1alpha1/informers/externalversions"
	sanclientset "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	saninformers "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/informers/externalversions"
	lvmclientset "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/clientset/versioned"
	lvminformers "github.com/upmio/dbscale-kube/pkg/client/volumepath/v1alpha1/informers/externalversions"
	"github.com/upmio/dbscale-kube/pkg/vars"
	v1beta1apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

var (
	versionFlag bool
	masterURL   string
	kubeconfig  string

	script = "/opt/kube/scripts/StorMGR/StorMGR"
)

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	rand.Seed(time.Now().UTC().UnixNano())
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Info("VERSION: ", vars.GITCOMMIT, " ", vars.BUILDTIME)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err)
	}

	err = initCRDs(cfg)
	if err != nil {
		klog.Errorf("Error init crds: %s", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err)
	}

	lvmclient, err := lvmclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err)
	}

	sanclient, err := sanclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err)
	}

	hostClient, err := hostclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	lvminformer := lvminformers.NewSharedInformerFactory(lvmclient, time.Second*30)
	saninformer := saninformers.NewSharedInformerFactory(sanclient, time.Second*30)
	hostinformer := hostinformers.NewSharedInformerFactory(hostClient, time.Second*30)

	ctrl := sanctrlv1.NewController(
		vars.SeCretAESKey, script,
		kubeClient, sanclient, hostClient, lvmclient,
		kubeInformerFactory.Core().V1(),
		saninformer.San().V1alpha1(),
		hostinformer.Host().V1alpha1().Hosts(),
		lvminformer.Lvm().V1alpha1().VolumePaths())

	go kubeInformerFactory.Start(stopCh)
	go lvminformer.Start(stopCh)
	go saninformer.Start(stopCh)
	go hostinformer.Start(stopCh)

	go ctrl.Run(1, stopCh)

	<-stopCh
	klog.Fatal("Never running where !!!")
	os.Exit(1)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
	flag.StringVar(&script, "scripts", script, "path to storage script dir.")
}

func initCRDs(config *rest.Config) error {
	client, err := apiextensions.NewForConfig(config)
	if err != nil {
		return err
	}

	errs := make([]error, 0, 4)

	_, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), &v1beta1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "sansystems." + sanv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1beta1apiextensions.CustomResourceDefinitionSpec{
			Group: sanv1alpha1.SchemeGroupVersion.Group,
			Names: v1beta1apiextensions.CustomResourceDefinitionNames{
				Kind:     "SanSystem",
				ListKind: "SanSystemList",
				Plural:   "sansystems",
			},
			Scope:   v1beta1apiextensions.ClusterScoped,
			Version: sanv1alpha1.SchemeGroupVersion.Version,
			Subresources: &v1beta1apiextensions.CustomResourceSubresources{
				Status: &v1beta1apiextensions.CustomResourceSubresourceStatus{},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), &v1beta1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "lungroups." + sanv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1beta1apiextensions.CustomResourceDefinitionSpec{
			Group: sanv1alpha1.SchemeGroupVersion.Group,
			Names: v1beta1apiextensions.CustomResourceDefinitionNames{
				Kind:     "Lungroup",
				ListKind: "LungroupList",
				Plural:   "lungroups",
			},
			Scope:   v1beta1apiextensions.NamespaceScoped,
			Version: sanv1alpha1.SchemeGroupVersion.Version,
			Subresources: &v1beta1apiextensions.CustomResourceSubresources{
				Status: &v1beta1apiextensions.CustomResourceSubresourceStatus{},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	//apiVersion: apiextensions.k8s.io/v1beta1
	//kind: CustomResourceDefinition
	//metadata:
	//  name: volumepaths.lvm.upm.io
	//spec:
	//  group: lvm.upm.io
	//  names:
	//    kind: VolumePath
	//    listKind: VolumePathList
	//    plural: volumepaths
	//  scope: Namespaced
	//  version: v1alpha1

	_, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), &v1beta1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "volumepaths." + lvmv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1beta1apiextensions.CustomResourceDefinitionSpec{
			Group: lvmv1alpha1.SchemeGroupVersion.Group,
			Names: v1beta1apiextensions.CustomResourceDefinitionNames{
				Kind:     "VolumePath",
				ListKind: "VolumePathList",
				Plural:   "volumepaths",
			},
			Scope:   v1beta1apiextensions.NamespaceScoped,
			Version: lvmv1alpha1.SchemeGroupVersion.Version,
			Subresources: &v1beta1apiextensions.CustomResourceSubresources{
				Status: &v1beta1apiextensions.CustomResourceSubresourceStatus{},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
