# FC 默认角色：允许 FC 服务挂载弹性网卡访问 VPC（应用级）
resource "alicloud_ram_role" "fc" {
  role_name                   = "${local.app_name_prefix}-fc"
  assume_role_policy_document = <<EOF
{
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": ["fc.aliyuncs.com"]
      }
    }
  ],
  "Version": "1"
}
EOF
  description                 = "Function Compute 默认角色（qtcloud-pay）"
}

resource "alicloud_ram_role_policy_attachment" "fc_vpc" {
  policy_name = "AliyunECSNetworkInterfaceManagementAccess"
  policy_type = "System"
  role_name   = alicloud_ram_role.fc.role_name
}

# 函数计算（FC 3.0）：custom-container 容器镜像，VPC 内网访问 RDS（应用级）
resource "alicloud_fcv3_function" "this" {
  function_name     = local.app_name_prefix
  description       = "qtcloud-pay 账本核心 API"
  runtime           = "custom-container"
  handler           = "index.handler" # custom-container 必填占位，实际由容器监听端口决定
  cpu               = 0.5
  memory_size       = var.fc_memory
  timeout           = var.fc_timeout
  internet_access   = true
  role              = alicloud_ram_role.fc.arn
  resource_group_id = local.resource_group_id

  vpc_config {
    vpc_id            = alicloud_vpc.this.id
    vswitch_ids       = [alicloud_vswitch.this.id]
    security_group_id = alicloud_security_group.this.id
  }

  custom_container_config {
    image = var.image
    port  = 8080
  }

  # 对齐 provider 运行时约定：DB_DRIVER=postgres + DATABASE_URL（见 internal/app/app.go）
  # 注意：密码会以明文落入 tfstate，生产环境建议改用 FC 配置中心/密钥管理注入
  environment_variables = {
    DB_DRIVER    = "postgres"
    DATABASE_URL = "postgres://${alicloud_db_account.this.account_name}:${var.db_password}@${alicloud_db_instance.this.connection_string}:${alicloud_db_instance.this.port}/${alicloud_db_database.this.data_base_name}?sslmode=disable"
  }

  tags = {
    project     = var.project
    environment = var.environment
  }
}

# HTTP 触发器：使服务可直接访问（后续由系统级 API 网关统一接入，此触发器保留为直连通道）
resource "alicloud_fcv3_trigger" "http" {
  function_name = alicloud_fcv3_function.this.function_name
  trigger_name  = "http"
  trigger_type  = "http"
  qualifier     = "LATEST"
  trigger_config = jsonencode({
    authType = "anonymous"
    methods  = ["GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"]
  })
}
