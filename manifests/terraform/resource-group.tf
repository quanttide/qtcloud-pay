# 资源组：所有资源统一归入 quanttide（系统级共享，权限与成本账单按组管控）
# 已存在则复用（data source），不存在则创建
data "alicloud_resource_manager_resource_groups" "quanttide" {
  name_regex = "^quanttide$"
}

resource "alicloud_resource_manager_resource_group" "quanttide" {
  count               = length(data.alicloud_resource_manager_resource_groups.quanttide.groups) == 0 ? 1 : 0
  display_name        = "quanttide"
  resource_group_name = "quanttide"
}

locals {
  resource_group_id = length(data.alicloud_resource_manager_resource_groups.quanttide.groups) > 0 ? data.alicloud_resource_manager_resource_groups.quanttide.groups[0].id : alicloud_resource_manager_resource_group.quanttide[0].id
}
