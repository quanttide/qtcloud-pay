# 系统级共享网络：RDS 与 FC 通过 VPC 内网互通（quanttide 体系统一管理）
data "alicloud_zones" "default" {
  available_resource_creation = "Rds"
}

resource "alicloud_vpc" "this" {
  vpc_name   = local.name_prefix
  cidr_block = var.vpc_cidr
}

resource "alicloud_vswitch" "this" {
  vpc_id       = alicloud_vpc.this.id
  cidr_block   = var.vswitch_cidr
  zone_id      = data.alicloud_zones.default.zones[0].id
  vswitch_name = local.name_prefix
}

# FC 挂载用安全组（RDS 走白名单而非安全组，见 rds.tf）
resource "alicloud_security_group" "this" {
  security_group_name = local.name_prefix
  vpc_id              = alicloud_vpc.this.id
}
