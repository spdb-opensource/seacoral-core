# kube-volumepath
# 注册CRD资源
- 目前手动向k8s注册资源: artifacts/hostpath-crd.yaml  

# 环境搭建
 1. 启动：  ./kubeseed -kubeconfig $HOME/.kube/config --logtostderr=true -v=4  -hostname=xxx -shellDir=/tmp/scripts/
    - 注明：hostname参数必填，且要跟kubelet的--hostname-override 参数值一致。
 
 2. 使用最新sriov网络插件， 并修改配置sriov.conf 添加"volumepathCheck":true
    - 作用： 避免pod起来后，挂载点目录存在而设备还没mount该挂载点，导致使用本地存储资源。

# 使用
## 创建：
  以 artifacts/hostpath.yaml模板配置好.
    - 创建 kubectl create -f hostpath.yaml
	- 查看 kubectl describe volumepath test1

## deactivate:
  - kubectl edit volumepath test1   修改对应spec node字段为"". 

##  失败重做：
  - 作用: 创建，扩展，activate/deactivate 失败后设置该值可触发重做
  - 查看 kubectl describe volumepath test1,如果状态失败，则 kubectl edit volumepath test1 : 添加spec字段并设置: actCode: 111
  
## clean:
  - 原目标主机清理
  - kubectl edit volumepath test1 : 添加spec字段并设置: actCode: 222

##  activate:
  -  kubectl edit volumepath test1  修改spec node字段为目标主机(eg：172.16.109.133）

## 删除：
  - kubectl edit volumepath test1 : 添加spec字段并设置: actCode: 943

 