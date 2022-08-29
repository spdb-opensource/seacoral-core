# how to schedule pods?
##1. goal
  we want k8s schedule pods to the nodes equality, and we want to remain some resources to expansion purpose, thus when only few resources left on the node, no new pods should be scheduled, but pods which were already scheduled to this node before, can still be scheduled to this node, no matter how many resources left.

##2. mark labels
  when adding an app, CM-apiserver will check how many resources left on each host, it will compare allocatable resources(cpu, memory, disk) with total capacity, and mark labels on the host with the following rules:  
a. if any one of allocatable resources is less than **resourceLowWaterMark**(default 10%), the host will be marked resource.allocatable.level=low,  
b. if all allocatable resources are higher than **resourceMediumWaterMark**(default 50%), the host will be marked resource.allocatable.level=high,  
c. otherwise, the host will be marked resource.allocatable.level=medium.  

##3. set node affinity
after marking labels on each dbscale-node hosts, the pod will be set **PreferredDuringSchedulingIgnoredDuringExecution** node affinity with expression that tells k8s to schedule pods to the nodes with label resource.allocatable.level equals to medium or high.

