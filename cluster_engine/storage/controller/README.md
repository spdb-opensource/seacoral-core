# SanStorage

### 启动
	./san-storage -kubeconfig $HOME/.kube/config
	
	.kube/config 本地 kubectl 配置目录

### 注册SAN存储	
	kubectl apply -f artifacts/sanv1alpha2/sansystem.yaml

### 注册Host
	kubectl apply -f artifacts/sanv1alpha2/host.yaml
	
### 创建PVC
	kubectl create -f artifacts/sanv1alpha2/pvc.yaml
	
### 创建Pod
	kubectl create -f artifacts/sanv1alpha2/pod.yaml
	
