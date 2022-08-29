# kube-sriov
kubernetes networking manager based on sriov

# 需求
 - 大二层网络，需要多个网络段管理
 - pod迁移，重启等IP不能变 
 - 基于SR-IOV,需要其带宽，vlan等特性

# 设计
- [设计方案](./design.md)
-  network名字空间唯一（类比 k8s volume）

# plugin 执行环境要求
- 使用支持sriov的物理网卡

# plugin 环境搭建
1. 配置添加kubelet启动参数：

  --network-plugin=cni --cni-conf-dir=/etc/cni/net.d --cni-bin-dir=/root/local/bin 

2. 将sriov.conf配置放入/etc/cni/net.d 目录下:

  cat sriov.conf查看，配置相关参数：
        "shellDir":"/tmp/scripts/“,     //脚本存放位置
        "kubeconfig":"$HOME/.kube/config” //kubeconfig配置位置，要与kubelet启动参数的—kubeconfig配置值一样。
        "noCheckVolumepath":false ,//默认会检查volumepath挂载卷是否挂载，设置为true则不检查


3. 将二进制loopback和sriov放入/root/local/bin目录下


4. 将脚本getNetDevice.sh和addNetwork.sh放入sriov.conf中shellDir配置的位置


5. 重启kubelet

# 实验使用
2. master节点上： 
  启动networkcontroller（networkcontroller启动后，会自动注册network和networkClaim.）: 
    cd networkcontroller && go build && ./networkcontroller -kubeconfig $HOME/.kube/config --logtostderr=true -v=4

3. 创建pod流程 
 - 创建网络资源池： 
     -  以network1.yaml模板配置好网段 
     -  kubectl create -f   network1.yaml 

  - 创建networkcliam: 
      -  以 networkcliam1模板配置修改（networkBelong字段来指定使用哪个网络资源池） 
      - kubectl create -f   networkcliam1.yaml 

 - 创建pod: 
    - 创建pod时，通过labels指定使用那个networkcliam,eg: 
        labels: 
		 upm.networkClaim.internal: networkcliam1   必填  内网IP
		 upm.networkClaim.external: networkcliam2   可选  外网IP
  （注意：pod必须调度到kubelet使用sriov网络插件的宿主机上） 

# 查看 
- kubectl get networks --all-namespaces 
- kubectl describe networkClaims 
- kubectl describe networkClaims/networkcliam1 

# 排错 
- journalctl -f -u kubelet 
