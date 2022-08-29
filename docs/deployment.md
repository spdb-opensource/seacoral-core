# 1. dbch 软件和硬件环境建议配置

## 1.1 Linux 操作系统版本要求

| **Linux 操作系统平台**   | **版本**  |
| ------------------------ | --------- |
| Red Hat Enterprise Linux | 7.9及以上 |
| CentOS                   | 7.9及以上 |
                                            

## 1.2 中控机软件配置

| **软件**      | **版本**          |
| ------------- | ----------------- |
| docker-ce     | 19.03.14-3 及以上 |
| docker-ce-cli | 19.03.14-3 及以上 |

## 1.3 开发及测试环境建议配置

### 1.3.1资源要求

**注意事项：**

**使用kubespray注意事项：主机名必须全部都是小写**

| **角色**               | **Cpu/Mem（最小）** | **磁盘（最小）**                |
| ---------------------- | ------------------- | ------------------------------- |
| 运维管理备份节点       | 2核4G               | 60G（HDD）系统                  |
| kubernetes集群管理节点 | 4核8G               | 100G（SSD）系统                 |
| 集群监控节点           | 2核4G               | 60G（HDD）系统                  |
| 多集群日志节点         | 4核8G               | 60G（HDD）系统/100G （HDD）数据 |
| 镜像仓库节点           | 2核4G               | 60G（HDD）系统/100G （HDD）数据 |
| 计算节点               | 4核8G               | 60G（HDD）系统/100G （HDD）数据 |
| 多集群管理节点         | 4核8G               | 100G（SSD）系统                 |

### 1.3.2 网络要求

| **组件**             | **默认端口** | **说明**                                 |
| -------------------- | ------------ | ---------------------------------------- |
| dbch                 | 8088         | 应用访问通信端口                         |
| Prometheus           | 30001        | Prometheus 服务通信端口                  |
| Grafana              | 30000        | 监控服务对外服务和客户端(浏览器)访问端口 |
| Cluster manager      | 8082         | Cluster manager 服务通信端口             |
| Cluster engine       | 20152        | Cluster engine服务通信端口               |
| Elasticsearch        | 9200         | 日志收集收索(浏览器)访问端口             |
| kibana               | 5601         | 日志展示(浏览器)访问端口                 |
| kubernetes-dashboard | 30002        | K8s集群控制展示(浏览器)访问端口          |
| Mysql                | 3306         | mysql数据库默认端口                      |

# 2. 在离线环境部署 dbscale

**以下所有操作均在中控机上完成**

## 2.1获取介质

| **介质**                               | **说明**                       |
| -------------------------------------- | ------------------------------ |
| dbch-spdb-deploy-v1.0.0-694822c.tar.gz | 以下所有介质的压缩包           |
| dbch-dbch-spdb-deploy-v1.0.0.tar       | 镜像                           |
| inventory.ini.tmp                      | 主机列表及变量定义文件模版文件 |
| deploy.sh                              | 自动化安装部署脚本             |
| images                                 | 镜像目录                       |
| packages                               | 软件介质目录                   |
| README.md                              | 安装手册                       |
| reset.sh                               | 卸载脚本                       |
| update.sh                              | 升级脚本                       |


## 2.2 准备安装介质

将安装介质传到ansible中控机节点/root/目录下，并解压。
```bash
cd /root/

tar -xf dbch-spdb-deploy-v1.0.0-694822c.tar.gz
```

docker读取images
```bash
cd dbch-spdb-deploy-v1.0.0-694822c

docker load -i dbscale-kube-deploy-v1.0.0.tar
```

## 2.3 准备inventory文件

复制模版文件生产部署环境的inventory文件
```bash
cp inventory.ini.tmp ./inventory.ini
```

编写inventory文件
```yml
[consul]

#${consul1_ip}

#${consul2_ip}

#${consul3_ip}

[mysql]

#${mysql1_ip}

#${mysql2_ip}

#${mysql3_ip}

[web_manager]

#${web1_ip}

#${web2_ip}

#${web3_ip}

[cluster_manager]

#${cm1_ip}

#${cm2_ip}

#${cm3_ip}

[harbor]

#${harbor_ip}

[kube-master]

#${master1_ip}

#${master2_ip}

#${master3_ip}

[kube-node]

#${node1_ip}

#${node2_ip}

#${node3_ip}

[prometheus]

#${prometheus_ip}

[elasticsearch]

#${elk1_ip}

#${elk2_ip}

#${elk3_ip}

[kibana]

#${kibana_ip}

[filebeat:children]

elasticsearch

kibana

consul

mysql

web_manager

cluster_manager

[all:vars]

#介质目录

packages_dir="{{ playbook_dir }}/packages"

#k8s集群名

cluster_name="mycluster1"

#k8s集群对外域名

cluster_domain="{{ cluster_name }}.service.consul"

#集群描述

site_desc="站点1"

site_region="SH"

created_user="user001-用户"

kube_pods_subnet="10.233.64.0/18"

#cluster_engine 端口号

ce_port=20152

#cluster_manager 家目录

cm_home_dir="/opt/cluster_manager"

#cluster_manager 二进制目录

cm_bin_dir="{{ cm_home_dir }}/bin"

#cluster_manager 配置文件目录

cm_conf_dir="{{ cm_home_dir }}/conf"

#cluster_manager 端口号

cm_port=8082

#cluster_manager 域名

cm_name="cluster_manager"

cm_dns_domain="{{ cm_name }}.service.consul"

#cluster_manager 数据库相关信息

cm_db_name="cluster_manager"

cm_db_user="db_cm"

cm_db_passwd="12#$56"

#cmha 家目录

cmha_home_dir="/opt/cmha"

#cmha 二进制存放目录

cmha_bin_dir="{{ cmha_home_dir }}/bin"

#switchmanager 家目录

sm_home_dir="/opt/switchmanager"

#switchmanager 二进制存放目录

sm_bin_dir="{{ sm_home_dir }}/bin"

#switchmanager 配置文件目录

sm_conf_dir="{{ sm_home_dir }}/conf"

#switchmanager 日志文件目录

sm_log_dir="{{ sm_home_dir }}/log"

#switchmanager 脚本目录

sm_script_dir="{{ sm_home_dir }}/script"

#switchmanager 端口

sm_port=9201

#consul 数据目录

consul_data_dir=/var/lib/consul

#consul 数据中心名字

consul_datacenter=dbscale

#harbor https端口

harbor_https_port=28083

#harbor http端口

harbor_http_port=8080

#harbor管理员密码

harbor_password="12#$56"

#harbor后台数据库密码

harbor_db_password="12#$56"

#harbor日志等级

harbor_log_level="info"

#harbor名字

harbor_prefix="registry1"

#harbor域名

harbor_domain="{{ harbor_prefix }}.harbor.{{ cluster_domain }}"

#harbor家目录

harbor_home_dir="/harbor"

#harbor数据目录

harbor_data_dir="{{ harbor_home_dir }}/data"

#harbor日志目录

harbor_log_dir="{{ harbor_home_dir }}/log"

#harbor安装目录

harbor_install_dir="{{ harbor_home_dir }}/install"

#harbor证书目录

harbor_cert_dir="{{ harbor_data_dir }}/cert"

#harbor证书文件

harbor_cert_file="{{ harbor_cert_dir }}/{{ harbor_domain }}.crt"

#harbor私钥文件

harbor_key_file="{{ harbor_cert_dir }}/{{ harbor_domain }}.key"

#kubespray使用镜像项目名称

k8s_project="k8s"

#promeetheus使用镜像项目名称

prom_project="prom"

#elk使用镜像项目名称

elk_project="elk"

#es家目录

elasticsearch_home_dir="/opt/elasticsearch"

#es数据目录

elasticsearch_data_dir="{{ elasticsearch_home_dir }}/data"

#es日志目录

elasticsearch_log_dir="{{ elasticsearch_home_dir }}/log"

#es集群名称

elasticsearch_cluster_name="cluster-es"

#mysql 数据目录

mysql_data_dir="/var/lib/mysql"

#mysql 高可用域名

mysql_name="mysql"

mysql_dns_domain="{{ mysql_name }}.service.consul"

#mysql root用户密码

mysql_root_password="12#$56"

#mysql 复制用户名和密码

mysql_repl_user="repl"

mysql_repl_password="12#$56"

#mysql 检查用户名和密码

mysql_check_user="cmha_check"

mysql_check_password="12#$56"

#mysql 端口

mysql_port=3306

#web_manager 家目录

web_home_dir="/opt/web_manager"

#web_manager 二进制目录

web_bin_dir="{{ web_home_dir }}/bin"

#web_manager 配置文件目录

web_conf_dir="{{ web_home_dir }}/conf"

#web_manager tomcat 配置文件目录

web_tomcat_conf_dir="{{ web_home_dir }}/tomcat/conf"

#web_manager 日志目录

web_log_dir="{{ web_home_dir }}/log"

#web_manager war包目录

web_war_dir="{{ web_home_dir }}/war"

#web_manager 端口号

web_port=8088

#web_manager 数据库相关信息

web_db_name="dbscale"

web_db_user="db_api"

web_db_passwd="12#$56"

#web 镜像

web_image="dbscale/dbscale-kube-web:2009-centos7.6-amd64"

#POC data_example

NETWORK1_END="192.168.26.120"

NETWORK1_GATEWAY="192.168.21.1"

NETWORK1_PREFIX=20

NETWORK1_START="192.168.26.101"

NETWORK1_VLAN=0

NETWORK2_END="192.168.26.140"

NETWORK2_GATEWAY="192.168.21.1"

NETWORK2_PREFIX=20

NETWORK2_START="192.168.26.121"

NETWORK2_VLAN=0
```

## 2.4 免密
在宿主机上做免密即可，运行容器时会将宿主机的id_rsa传入容器中
```yml
ssh-keygen -t rsa

ssh-copy-id ${harbor_ip}

ssh-copy-id ${master1_ip}

ssh-copy-id ${master2_ip}

ssh-copy-id ${master3_ip}

ssh-copy-id ${node1_ip}

ssh-copy-id ${node2_ip}

ssh-copy-id ${node3_ip}

ssh-copy-id ${promtheus_ip}

ssh-copy-id ${elk1_ip}

ssh-copy-id ${elk2_ip}

ssh-copy-id ${elk3_ip}

ssh-copy-id ${kibana_ip}
```

## 2.5 设置环境变量步骤

设置环境变量
```yml
export DBSCALE_INVENTORY_FILE=/root/dbch-spdb-deploy-v1.0.0-694822c/inventory.ini

export CERTS_DIR=/root/dbch-spdb-deploy-v1.0.0-694822c/certs

export DBSCALE_PACKAGES=/root/dbch-spdb-deploy-v1.0.0-694822c/packages

export DBSCALE_IMAGES=/root/dbch-spdb-deploy-v1.0.0-694822c/images
```

运行部署脚本：
```yml
sh deploy.sh all
```
