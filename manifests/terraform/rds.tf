# 服务关联角色：RDS PostgreSQL 首次开通前必须存在（账号级一次性前置）
resource "alicloud_rds_service_linked_role" "pgsql" {
  service_name = "AliyunServiceRoleForRdsPgsql"
}

# 系统级共享 RDS PostgreSQL Serverless（按量计费、自动暂停；单节点 serverless_basic）
# 实例为 quanttide 体系共享（多应用共用一个实例，各自独立数据库）；qtcloud-pay 的应用库见下方 database 资源
resource "alicloud_db_instance" "this" {
  engine                   = "PostgreSQL"
  engine_version           = var.db_engine_version
  category                 = var.db_category
  instance_type            = "pg.n2.serverless.1c"
  instance_storage         = 20
  db_instance_storage_type = "cloud_essd"
  vswitch_id               = alicloud_vswitch.this.id
  port                     = "5432"
  # 白名单：仅允许 VPC 交换机网段内网访问（FC 通过 VPC 访问 RDS 内网地址）
  security_ips = [var.vswitch_cidr]
  serverless_config {
    min_capacity = var.db_min_capacity
    max_capacity = var.db_max_capacity
    auto_pause   = true
    switch_force = false
  }
  instance_name     = local.name_prefix
  resource_group_id = local.resource_group_id
  tags = {
    project     = var.project
    environment = var.environment
  }
}

resource "alicloud_db_database" "this" {
  instance_id    = alicloud_db_instance.this.id
  data_base_name = var.db_name
  character_set  = "UTF8"
}

resource "alicloud_db_account" "this" {
  db_instance_id   = alicloud_db_instance.this.id
  account_name     = var.db_username
  account_password = var.db_password
  account_type     = "Normal"
}

resource "alicloud_db_account_privilege" "this" {
  instance_id  = alicloud_db_instance.this.id
  account_name = alicloud_db_account.this.account_name
  privilege    = "ReadWrite"
  db_names     = [alicloud_db_database.this.data_base_name]
}
