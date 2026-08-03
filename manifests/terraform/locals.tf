locals {
  # 系统级共享资源（quanttide 体系统一管理）：VPC / 交换机 / 安全组 / RDS 实例
  name_prefix = "quanttide-${var.environment}"
  # 应用级资源：FC 函数 / RAM 角色
  app_name_prefix = "${var.project}-${var.environment}"
}
