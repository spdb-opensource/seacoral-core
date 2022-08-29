-- MySQL dump 10.13  Distrib 8.0.17, for macos10.14 (x86_64)
--
-- Host: 192.168.22.40    Database: cluster_manager
-- ------------------------------------------------------
-- Server version	5.7.25-log

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `tbl_app`
--
CREATE DATABASE IF NOT EXISTS `cluster_manager`;
USE `cluster_manager`;
DROP TABLE IF EXISTS `tbl_app`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_app` (
  `id` varchar(64) NOT NULL COMMENT '服务应用唯一标识符',
  `name` varchar(128) NOT NULL COMMENT '服务应用名称，用于展示。',
  `spec` text NOT NULL COMMENT '需求描述信息，使用JSON格式存储',
  `subscription_id` varchar(128) DEFAULT NULL COMMENT '订阅号',
  `spinlock` int(11) NOT NULL,
  `status_database` varchar(128) NOT NULL  COMMENT '存放database readiness status',
  `status_cmha` varchar(128) NOT NULL  COMMENT '存放cmha readiness status',
  `status_proxysql` varchar(128) NOT NULL  COMMENT '存放proxysql readiness status',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_backup_file`
--

DROP TABLE IF EXISTS `tbl_backup_file`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_backup_file` (
  `id` varchar(64) NOT NULL,
  `type` varchar(32) NOT NULL,
  `strategy_id` varchar(45) NOT NULL,
  `job_name` varchar(128) NOT NULL,
  `site_id` varchar(64) NOT NULL,
  `namespace` varchar(32) NOT NULL,
  `file` varchar(128) NOT NULL,
  `endpoint_id`  varchar(64) NOT NULL,
  `app_id` varchar(64) NOT NULL,
  `unit_id` varchar(64) NOT NULL,
  `task_id` varchar(64) NOT NULL,
  `size` int(11) DEFAULT NULL,
  `status` varchar(32) NOT NULL COMMENT '备份动作状态。枚举值范围：success, creating ,failed',
  `expired_timestamp` timestamp NULL DEFAULT NULL COMMENT '备份文件过期时间。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间，用于展示。',
  `finished_timestamp` timestamp NULL DEFAULT NULL COMMENT '完成时间，用于展示。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_backup_strategy`
--

DROP TABLE IF EXISTS `tbl_backup_strategy`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_backup_strategy` (
  `id` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `retention` int(11) NOT NULL COMMENT '备份策略有效天数。单位：天数',
  `type` varchar(32) NOT NULL COMMENT '备份类型。枚举范围：full',
  `active` tinyint(4) NOT NULL COMMENT '是否活动。值范围: true = 1, false = 0',
  `app_id` varchar(64) NOT NULL COMMENT '指定备份对象服务ID。',
  `unit_id` varchar(64) DEFAULT NULL COMMENT '指定备份对象单元ID。',
  `endpoint_id` varchar(64) DEFAULT NULL COMMENT '指定备份存储终端ID',
  `role` varchar(32) DEFAULT NULL COMMENT '指定备份对象角色。枚举范围：master, slave',
  `tables` varchar(512) DEFAULT NULL,
  `schedule` varchar(32) NOT NULL,
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_cluster`
--

DROP TABLE IF EXISTS `tbl_cluster`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_cluster` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `site_id` varchar(64) NOT NULL COMMENT '关联的站点唯一标识符。',
  `name` varchar(128) NOT NULL COMMENT '站点名称，用于展示。',
  `zone` varchar(64) NOT NULL COMMENT '集群所属区域标签，用于展示分类。',
  `image_type` varchar(256) NOT NULL COMMENT '集群可用的镜像类型，用于资源选择管理。值规范：允许支持多个镜像类型，使用“,”间隔。举例：mysql,redis',
  `ha_tag` varchar(64) NOT NULL COMMENT '集群高可用标签，用于资源选择管理。',
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_site_id_name` (`site_id`,`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_host`
--

DROP TABLE IF EXISTS `tbl_host`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_host` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `host_name` varchar(64) NOT NULL COMMENT '主机Hostname',
  `host_ip` varchar(64) NOT NULL COMMENT '主机IP地址',
  `cluster_id` varchar(64) NOT NULL COMMENT '所属集群唯一标识符。',
  `room` varchar(128) NOT NULL COMMENT '主机所在机房',
  `seat` varchar(128) NOT NULL COMMENT '主机所在机架位',
  `storage_remote_id` varchar(64) DEFAULT NULL COMMENT '主机所链外置存储ID',
  `max_usage_cpu` int(11) NOT NULL,
  `max_usage_mem` int(11) NOT NULL,
  `max_usage_bandwidth` int(11) NOT NULL,
  `unit_max` int(11) NOT NULL COMMENT '最大单元数量。',
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`),
  UNIQUE KEY `host_ip_UNIQUE` (`host_ip`),
  KEY `idx_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_image`
--

DROP TABLE IF EXISTS `tbl_image`;
/*!40101 SET @saved_cs_client = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_image`
(
    `id`                 varchar(64) NOT NULL COMMENT '唯一标识符。',
    `site_id`            varchar(64) NOT NULL COMMENT '镜像所在站点id',
    `type`               varchar(32) NOT NULL COMMENT '镜像类型。枚举值范围：mysql',
    `arch`               varchar(32) NOT NULL COMMENT 'ARCH',
    `version_major`      int(11)     NOT NULL COMMENT '主版本号',
    `version_minor`      int(11)     NOT NULL COMMENT '次版本号',
    `version_patch`      int(11)     NOT NULL COMMENT '修订版本号',
    `version_build`      int(11)     NOT NULL DEFAULT '0' COMMENT '编译版本号',
    `exporter_port`      int(11)      NOT NULL DEFAULT '0' COMMENT 'exporter_port',
    `unschedulable`      tinyint(4)  NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
    `key_sets`           text                 DEFAULT NULL COMMENT 'key_set',
    `config_template`    text                 DEFAULT NULL COMMENT 'config_template。',
    `pod_template`       text                 DEFAULT NULL COMMENT 'pod_template。',
    `description`        varchar(512)         DEFAULT NULL COMMENT '描述信息。',
    `created_timestamp`  timestamp   NULL     DEFAULT NULL COMMENT '创建时间，用于展示。',
    `modified_timestamp` timestamp   NULL     DEFAULT NULL COMMENT '修改时间，用于展示。',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_version` (`type`, `version_major`, `version_minor`, `version_build`, `version_patch`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_network`
--

DROP TABLE IF EXISTS `tbl_network`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_network` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `name` varchar(128) NOT NULL COMMENT '网段名称，用于展示。',
  `cluster_id` varchar(64) NOT NULL COMMENT '所属集群唯一标识符。',
  `topology` varchar(128) NOT NULL COMMENT '网段的连通性拓扑标签，用于资源选择管理。值规范：允许支持多个，使用“,”间隔。举例：topo001,topo002',
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_id_name_UNIQUE` (`cluster_id`,`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_site`
--

DROP TABLE IF EXISTS `tbl_site`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_site` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `name` varchar(128) NOT NULL COMMENT '站点名称，用于展示。',
  `type` varchar(64) NOT NULL COMMENT '站点类型，用于表示cluster_engine类型。枚举值范围: kubernetes',
  `domain` varchar(128) NOT NULL COMMENT '站点链接域名，用于链接认证使用。',
  `port` int(11) NOT NULL COMMENT '站点链接端口，用于链接认证使用。',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `region` varchar(64) NOT NULL COMMENT '站点所在地理位置名称，用于展示。',
  `config` text NOT NULL COMMENT './kube/kubeconfig 文件内容',
  `image_registry` varchar(128) NOT NULL COMMENT '镜像仓库',
  `project_name` varchar(128) NOT NULL COMMENT '镜像项目名称',
  `network_mode`       varchar(64)  NOT NULL COMMENT '网络插件类型',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`),
  UNIQUE KEY `name_UNIQUE` (`name`),
  UNIQUE KEY `idx_domain_port_UNIQUE` (`domain`,`port`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站点表';
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_storage_host`
--

DROP TABLE IF EXISTS `tbl_storage_host`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_storage_host` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `host_id` varchar(64) NOT NULL COMMENT '所在主机ID',
  `name` varchar(64) NOT NULL COMMENT '名称',
  `performance` varchar(32) NOT NULL COMMENT '性能等级',
  `paths` varchar(256) NOT NULL COMMENT '设备路径列表',
  `max_usage` int(11) NOT NULL COMMENT '最大分配率，用于资源选择管理。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_storage_remote`
--

DROP TABLE IF EXISTS `tbl_storage_remote`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_storage_remote` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `name` varchar(128) NOT NULL COMMENT '存储名称，用于展示。',
  `site_id` varchar(64) NOT NULL COMMENT '关联的站点唯一标识符。',
  `vendor` varchar(64) NOT NULL COMMENT '存储设备厂商品牌，用于展示。',
  `model` varchar(64) NOT NULL COMMENT '存储设备型号，用于展示。',
  `type` varchar(64) NOT NULL COMMENT '存储链接类型。枚举值类型：fc, iscsi',
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_storage_remote_auth`
--

DROP TABLE IF EXISTS `tbl_storage_remote_auth`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_storage_remote_auth` (
  `storage_remote_id` varchar(64) NOT NULL COMMENT '关联的外置存储唯一标识符。',
  `auth_ip` varchar(64) NOT NULL COMMENT '认证 IP 地址。',
  `auth_port` int(11) NOT NULL COMMENT '认证端口。',
  `auth_username` varchar(64) NOT NULL COMMENT '认证用户名。',
  `auth_password` varchar(128) NOT NULL COMMENT '认证密码。',
  `auth_vstorename` varchar(64) NULL COMMENT '租户名称。',
  PRIMARY KEY (`storage_remote_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_storage_remote_pool`
--

DROP TABLE IF EXISTS `tbl_storage_remote_pool`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_storage_remote_pool` (
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `storage_remote_id` varchar(64) NOT NULL COMMENT '关联的外置存储唯一标识符。',
  `name` varchar(64) NOT NULL,
  `native_id` varchar(128) NOT NULL COMMENT '在存储系统中记录的唯一标识符。',
  `performance` varchar(32) NOT NULL COMMENT '性能等级，用于资源选择管理。枚举值范围：low, medium,high',
  `max_usage` int(11) NOT NULL COMMENT ' 最大分配率，用于资源选择管理。',
  `enabled` tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
  `description` varchar(512) DEFAULT NULL COMMENT '描述信息。',
  `created_user` varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
  `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
  `modified_user` varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
  `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_task`
--

DROP TABLE IF EXISTS `tbl_task`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_task` (
  `ai` int(11) NOT NULL AUTO_INCREMENT COMMENT '自增列',
  `id` varchar(64) NOT NULL COMMENT '唯一标识符。',
  `action` varchar(32) NOT NULL COMMENT '动作',
  `relate_id` varchar(64) NOT NULL COMMENT '关联对象ID',
  `relate_table` varchar(32) NOT NULL COMMENT '关联表名',
  `error` varchar(512) DEFAULT NULL COMMENT '错误信息',
  `status` int(7) NOT NULL COMMENT '状态',
  `created_user` varchar(32) NOT NULL COMMENT '创建用户',
  `created_at` timestamp(6) NULL DEFAULT NULL COMMENT '创建时间',
  `finished_at` timestamp(6) NULL DEFAULT NULL COMMENT '完成时间',
  PRIMARY KEY (`ai`),
  UNIQUE KEY `id_UNIQUE` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1160 DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `tbl_unit`
--

DROP TABLE IF EXISTS `tbl_unit`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_unit` (
  `id` varchar(64) NOT NULL COMMENT '单元唯一识别码，生产规则：<app_name>-<database/proxy/manager>-<uuid_8>',
  `site_id` varchar(64) NOT NULL COMMENT '所属站点ID',
  `namespace` varchar(64) NOT NULL COMMENT '所属站点的命名空间',
  `app_id` varchar(64) NOT NULL COMMENT '所在服务应用ID',
  `group_name` varchar(128) NOT NULL COMMENT '所在服务应用的分片组名称',
  `spinlock` int(11) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_site_id` (`site_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;

DROP TABLE IF EXISTS `tbl_backup_endpoint`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tbl_backup_endpoint` (
    `id`               varchar(64) NOT NULL COMMENT '唯一标识符。',
    `site_id`          varchar(64) NOT NULL COMMENT '关联的站点唯一标识符。',
    `name`             varchar(64) NOT NULL COMMENT 'Endpoint名称',
    `type`             varchar(64) NOT NULL COMMENT 'Endpoint类型',
    `endpoint_config`  varchar(1024) NOT NULL COMMENT '配置Json',
    `enabled`    tinyint(4) NOT NULL COMMENT '是否可用，用于资源选择管理。值范围: true = 1, false = 0',
    `created_user`      varchar(64) NOT NULL COMMENT '创建用户，用于展示。',
    `created_timestamp` timestamp NULL DEFAULT NULL COMMENT '创建时间，用于展示。',
    `modified_user`     varchar(64) DEFAULT NULL COMMENT '修改用户，用于展示。',
    `modified_timestamp` timestamp NULL DEFAULT NULL COMMENT '修改时间，用于展示。',
    PRIMARY KEY (`id`),
    UNIQUE KEY `id_UNIQUE` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1160 DEFAULT CHARSET=utf8mb4;
/*!40101 SET character_set_client = @saved_cs_client */;



/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2019-08-27 17:18:19
