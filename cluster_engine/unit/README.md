# unit-controller

### 启动

	./unit -kubeconfig $HOME/.kube/config  
	
	.kube/config 本地 kubectl 配置目录
	
	程序启动时会注册 Unit CRD
	
    程序依赖 storage、network 模块执行

### 创建Unit

	kubectl create -f artifacts/unitv1alpha2/unit.yaml
	
