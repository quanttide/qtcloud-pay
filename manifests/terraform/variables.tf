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
  default     = "prod"
}

variable "db_name" {
  description = "RDS 数据库名（创建在系统级共享实例上，实例由 quanttide-platform 管理）"
  type        = string
  default     = "qtcloud_pay"
}

variable "db_username" {
  description = "RDS 数据库账号名"
  type        = string
  default     = "qtcloud_pay"
}

variable "db_password" {
  description = "RDS 数据库账号密码（8-32 位，含大小写字母与数字）"
  type        = string
  sensitive   = true
}

variable "image" {
  description = "FC 容器镜像。由 CI 注入（TF_VAR_image 拼接 secret ALIYUN_ACR_REGISTRY 的实例地址）或 terraform.tfvars 提供；实例地址属敏感信息不写默认值"
  type        = string
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

variable "admin_token" {
  description = "运维删除端点保护令牌（X-Admin-Token）。由 CI 注入（TF_VAR_admin_token = secret ADMIN_TOKEN）；留空时端点返回 403（fail-closed）"
  type        = string
  sensitive   = true
  default     = ""
}

variable "secret_key" {
  description = "应用级 SECRET_KEY，用于支付服务端自签凭据校验。由 CI 注入（TF_VAR_secret_key = secret SECRET_KEY），缺失时服务启动失败"
  type        = string
  sensitive   = true
}

variable "auth_jwt_public_jwk" {
  description = "qtcloud-auth JWT 公钥 JWK/JWKS。配置后服务端可校验账号系统签发的 RS256 JWT；为空时仍依赖 SECRET_KEY 服务端凭据与 ADMIN_TOKEN"
  type        = string
  sensitive   = true
  default     = ""
}

variable "auth_jwt_issuer" {
  description = "qtcloud-auth JWT iss 校验值；为空则不校验 issuer"
  type        = string
  default     = ""
}

variable "auth_jwt_audience" {
  description = "qtcloud-auth JWT aud 校验值；为空则不校验 audience"
  type        = string
  default     = ""
}
