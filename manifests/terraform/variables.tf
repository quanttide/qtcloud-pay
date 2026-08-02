variable "region" {
  description = "阿里云地域"
  type        = string
  default     = "cn-hangzhou"
}

variable "project" {
  description = "项目名（资源命名前缀）"
  type        = string
  default     = "qtcloud-pay"
}

variable "environment" {
  description = "环境：dev / prod"
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "VPC 网段"
  type        = string
  default     = "10.0.0.0/16"
}

variable "vswitch_cidr" {
  description = "交换机网段（FC 与 RDS 内网互通）"
  type        = string
  default     = "10.0.1.0/24"
}

variable "db_name" {
  description = "RDS 数据库名"
  type        = string
  default     = "qtcloud_pay"
}

variable "db_username" {
  description = "RDS 数据库账号名"
  type        = string
  default     = "qtpay"
}

variable "db_password" {
  description = "RDS 数据库账号密码（8-32 位，含大小写字母与数字）"
  type        = string
  sensitive   = true
}

variable "db_engine_version" {
  description = "RDS PostgreSQL 版本（Serverless 基础版支持 14.0 / 15.0 / 16.0）"
  type        = string
  default     = "16.0"
}

variable "db_category" {
  description = "RDS 系列：serverless_basic（单节点，dev） / serverless_standard（高可用，prod）"
  type        = string
  default     = "serverless_basic"
}

variable "db_min_capacity" {
  description = "RDS Serverless 最小算力（RCU）"
  type        = number
  default     = 0.5
}

variable "db_max_capacity" {
  description = "RDS Serverless 最大算力（RCU）"
  type        = number
  default     = 4
}

variable "image" {
  description = "FC 容器镜像（需可公开拉取：Docker Hub 公开仓库或 ACR 公开镜像；私有 ACR 需控制台配置授权）"
  type        = string
  default     = "quanttide/qtcloud-pay:latest"
}

variable "fc_memory" {
  description = "FC 函数内存（MB）"
  type        = number
  default     = 512
}

variable "fc_timeout" {
  description = "FC 函数超时（秒）"
  type        = number
  default     = 60
}
