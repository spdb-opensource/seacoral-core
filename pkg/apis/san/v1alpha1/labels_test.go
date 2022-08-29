package v1alpha1

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ExampleStorageRequest() {
	claim := &corev1.PersistentVolumeClaim{}
	claim.SetLabels(map[string]string{StorageRequestLabel: "3Gi"})

	fmt.Println(StorageRequest(claim))

	// Output:
	// 3072 <nil>
}

func ExamplePVCStorageQuantity() {
	claim := &corev1.PersistentVolumeClaim{}
	claim.SetLabels(map[string]string{StorageRequestLabel: "3Gi"})

	q, err := resource.ParseQuantity("3071Mi")
	if err != nil {
		fmt.Println(err)
		return
	}

	p, err := PVCStorageQuantity(claim)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(p.Cmp(q))

	q1 := resource.NewQuantity(1024, resource.BinarySI)
	fmt.Println(q1.String())
	fmt.Println(q1.AsInt64())

	q2, err := resource.ParseQuantity("1024Mi")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(q2.String())
	fmt.Println(q2.AsInt64())
	v := q2.Value()
	fmt.Println(v, v>>20)

	// Output:
	// 1
	// 1Ki
	// 1024 true
	// 1Gi
	// 1073741824 true
	// 1073741824 1024
}

func ExampleLunsNotEqual() {
	a := []Lun{
		{
			Capacity:    100,
			Name:        "11111",
			StoragePool: "0",
		},
		{
			Capacity:    100,
			Name:        "1111",
			StoragePool: "1",
		},
	}

	b := []Lun{
		{
			Capacity:    100,
			Name:        "1111",
			StoragePool: "1",
		},
		{
			Capacity:    100,
			Name:        "11111",
			StoragePool: "0",
		},
	}

	c := []Lun{
		{
			Capacity:    100,
			Name:        "1111",
			StoragePool: "1",
		},
		{
			Capacity:    100,
			Name:        "111111",
			StoragePool: "0",
		},
	}

	d := []Lun{
		{
			Capacity:    100,
			Name:        "11111",
			StoragePool: "0",
		},
	}

	fmt.Println(LunsNotEqual(a, b))
	fmt.Println(LunsNotEqual(a, c))
	fmt.Println(LunsNotEqual(a, d))
	// Output:
	// false
	// true
	// true
}

func TestLungroupUpdate(t *testing.T) {
	info := LungroupInfo{
		Group:    "remote-pvc-5",
		Type:     AllocThick,
		NotExist: false,
		Luns: []Lun{
			{
				ID:          "291",
				Name:        "remote-pv-5-0",
				Capacity:    3072,
				StoragePool: "Iaas_ironic_01",
				Health:      "normal",
				Running:     "online",
				Desc:        "ERROR:null",
			},
		},
		Mapping: &HostgroupMapping{
			Name: "122.127.13.4",
			Hosts: []HostSpec{
				{
					Name: "122.127.13.4",
					IP:   "122.127.13.4",
				},
			},
		},
	}

	lg := &Lungroup{}
	update := pointerTest(lg, info)
	if !update {
		t.Error("expected true")
	}

	if len(lg.Status.Luns) != 1 {
		t.Errorf("expect len(lg.Status.Luns)==1),%v", lg)
	}

	if lg.Status.MappingView == nil {
		t.Error("expect lg.Status.MappingView != nil")
	}

	// failure test
	lg = &Lungroup{}
	update = pointerTest2(lg, info)
	if !update {
		t.Error("expected true")
	}

	if lg.Status.Luns != nil || lg.Status.MappingView != nil {
		t.Errorf("expect error but got %v", lg)
	}
}

func pointerTest(lg *Lungroup, info LungroupInfo) bool {
	clone := lg.DeepCopy()
	ok := clone.Update(info)
	clone.DeepCopyInto(lg)

	return ok
}

func pointerTest2(lg *Lungroup, info LungroupInfo) bool {
	clone := lg.DeepCopy()
	ok := clone.Update(info)
	lg = clone // lg, err = ctrl.clientset.SanV1alpha1().Lungroups(lg.Namespace).Update(lg)

	return ok
}
