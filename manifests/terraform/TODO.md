# Terraform 部署 TODO

## 上线前(阻塞项)

- [x] 配置阿里云凭证：本地 `~/.aliyun/config.json` + CI GitHub org secrets;RAM 用户授权 `PowerUserAccess` + `AliyunRAMFullAccess`
- [x] state 迁移到 OSS 远端后端（`quanttide-terraform-state`，已创建）
- [ ] 编写 `Dockerfile` 构建 provider 镜像并发布（Docker Hub / ACR），更新 `image` 变量
- [ ] 复制 `terraform.tfvars.example` → `terraform.tfvars` 填写真实值,执行 `terraform apply`
- [ ] 上线验证:通过 `terraform output fc_http_url` 访问账本 API(如健康检查/创建账户)确认全链路可用

## CI 与安全(P1)

- [ ] GitHub Actions workflow:terraform init / plan / apply,引用 org secrets(`ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`)
- [ ] 数据库密码不再明文落入 tfstate(改 FC 配置中心 / 密钥管理注入)
- [ ] state 迁移到 OSS 远端后端(`providers.tf` 已预留示例)

## 后续(P2)

- [ ] **OIDC 联邦升级**:RAM 角色信任 GitHub OIDC,workflow 不再需要 Secret Key,CI 不落任何长期凭证
- [ ] 环境划分 prod:RDS 系列切 `serverless_standard`(高可用),权限收敛为最小策略集
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-pay`(系统层面预留,另行规划)
