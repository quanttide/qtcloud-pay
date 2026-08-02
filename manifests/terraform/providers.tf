# 阿里云凭证通过环境变量注入（不在代码中写死）：
#   export ALICLOUD_ACCESS_KEY=...
#   export ALICLOUD_SECRET_KEY=...
provider "alicloud" {
  region = var.region
}

# 状态存储：默认本地。多环境/多人协作时切换为 OSS 远端后端：
# terraform {
#   backend "oss" {
#     bucket = "quanttide-terraform-state"
#     key    = "qtcloud-pay/terraform.tfstate"
#     region = "cn-hangzhou"
#   }
# }
