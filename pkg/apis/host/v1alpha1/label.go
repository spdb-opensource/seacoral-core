package v1alpha1

func GetVgCapacityLabel(name string) string {
	return name + ".capacity"
}

func GetVgAllocatableLabel(name string) string {
	return name + ".allocatable"
}
