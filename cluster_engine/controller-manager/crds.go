package main

import (
	"context"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	rest "k8s.io/client-go/rest"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking"
	sanv1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	lvmv1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	v1apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

var (
	//TODO: remove
	defaultPreserveUnknownFields = true
)

func initCRDs(config *rest.Config) error {
	client, err := apiextensions.NewForConfig(config)
	if err != nil {
		return err
	}

	errs := make([]error, 0, 9)
	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "units." + unitv4.SchemeGroupVersion.Group,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: unitv4.SchemeGroupVersion.Group,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:     "Unit",
				ListKind: "UnitList",
				Plural:   "units",
			},
			Scope: v1apiextensions.NamespaceScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    unitv4.SchemeGroupVersion.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "sansystems." + sanv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: sanv1alpha1.SchemeGroupVersion.Group,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:       "SanSystem",
				ListKind:   "SanSystemList",
				Plural:     "sansystems",
				ShortNames: []string{"san"},
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    sanv1alpha1.SchemeGroupVersion.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hosts." + hostv1.SchemeGroupVersion.Group,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: hostv1.SchemeGroupVersion.Group,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:     "Host",
				ListKind: "HostList",
				Plural:   "hosts",
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					AdditionalPrinterColumns: hostPrintColumnDefinition(),
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    hostv1.SchemeGroupVersion.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "lungroups." + sanv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: sanv1alpha1.SchemeGroupVersion.Group,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:       "Lungroup",
				ListKind:   "LungroupList",
				Plural:     "lungroups",
				ShortNames: []string{"lg"},
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    sanv1alpha1.SchemeGroupVersion.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "volumepaths." + lvmv1alpha1.SchemeGroupVersion.Group,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: lvmv1alpha1.SchemeGroupVersion.Group,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:       "VolumePath",
				ListKind:   "VolumePathList",
				Plural:     "volumepaths",
				ShortNames: []string{"vp"},
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					AdditionalPrinterColumns: volumepathPrintColumnDefinition(),
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    lvmv1alpha1.SchemeGroupVersion.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "networks." + networkv1.GroupName,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: networkv1.GroupName,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:     "Network",
				ListKind: "NetworkList",
				Plural:   "networks",
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					AdditionalPrinterColumns: networkPrintColumnDefinition(),
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    networkv1.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	_, err = client.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), &v1apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "networkclaims." + networkv1.GroupName,
		},
		Spec: v1apiextensions.CustomResourceDefinitionSpec{
			Group: networkv1.GroupName,
			Names: v1apiextensions.CustomResourceDefinitionNames{
				Kind:       "NetworkClaim",
				ListKind:   "NetworkClaimList",
				Plural:     "networkclaims",
				ShortNames: []string{"nc"},
			},
			Scope: v1apiextensions.ClusterScoped,
			Versions: []v1apiextensions.CustomResourceDefinitionVersion{
				v1apiextensions.CustomResourceDefinitionVersion{
					AdditionalPrinterColumns: networkClaimPrintColumnDefinition(),
					Schema: &v1apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &v1apiextensions.JSONSchemaProps{
							XPreserveUnknownFields: &defaultPreserveUnknownFields,
						},
					},
					Name:    networkv1.Version,
					Served:  true,
					Storage: true,
					Subresources: &v1apiextensions.CustomResourceSubresources{
						Status: &v1apiextensions.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func volumepathPrintColumnDefinition() []v1apiextensions.CustomResourceColumnDefinition {
	return []v1apiextensions.CustomResourceColumnDefinition{
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "type",
			Type:     "string",
			JSONPath: ".spec.type",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "SpecNode",
			Type:     "string",
			JSONPath: ".spec.node",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "CurNode",
			Type:     "string",
			JSONPath: ".status.bindingNode",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "SpecSize",
			Type:     "string",
			JSONPath: ".spec.size",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "CurSize",
			Type:     "string",
			JSONPath: ".status.curSize",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Status",
			Type:     "string",
			JSONPath: ".status.status",
		},
		// v1apiextensions.CustomResourceColumnDefinition{
		// 	Name:     "device",
		// 	Type:     "string",
		// 	JSONPath: ".status.device",
		// },
		// v1apiextensions.CustomResourceColumnDefinition{
		// 	Name:     "mounter",
		// 	Type:     "string",
		// 	JSONPath: ".status.mouter",
		// },
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Age",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}

func hostPrintColumnDefinition() []v1apiextensions.CustomResourceColumnDefinition {
	return []v1apiextensions.CustomResourceColumnDefinition{
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "phase",
			Type:     "string",
			JSONPath: ".status.phase",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Age",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}
func networkClaimPrintColumnDefinition() []v1apiextensions.CustomResourceColumnDefinition {
	return []v1apiextensions.CustomResourceColumnDefinition{
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "IP",
			Type:     "string",
			JSONPath: ".status.bindIP",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Used",
			Type:     "string",
			JSONPath: ".status.used",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "host",
			Type:     "string",
			JSONPath: ".status.host",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "device",
			Type:     "string",
			JSONPath: ".status.hostDevice",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Age",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}

func networkPrintColumnDefinition() []v1apiextensions.CustomResourceColumnDefinition {
	return []v1apiextensions.CustomResourceColumnDefinition{
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Start",
			Type:     "string",
			JSONPath: ".spec.startIP",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "End",
			Type:     "string",
			JSONPath: ".spec.endIP",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Counts",
			Type:     "integer",
			JSONPath: ".status.allIPCounts",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Used",
			Type:     "integer",
			JSONPath: ".status.usedIPCount",
		},
		v1apiextensions.CustomResourceColumnDefinition{
			Name:     "Age",
			Type:     "date",
			JSONPath: ".metadata.creationTimestamp",
		},
	}
}

func initCSIDriver(kubeClient kubernetes.Interface) error {
	_, err := kubeClient.StorageV1().CSIDrivers().Get(context.TODO(), lvmv1alpha1.VPCSIDriverName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err == nil {
		return nil
	}

	attachRequired := false
	vpdriver := &v1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: lvmv1alpha1.VPCSIDriverName,
		},
		Spec: v1.CSIDriverSpec{
			AttachRequired: &attachRequired,
		},
	}

	_, err = kubeClient.StorageV1().CSIDrivers().Create(context.TODO(), vpdriver, metav1.CreateOptions{})
	return err
}
