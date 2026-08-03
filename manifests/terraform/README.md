# qtcloud-pay 部署选型（IaC）

依据 2026-08-03 支付工程日志（[journal](../../../../data/journal/default/2026-08-02.md)）的部署决策总结，作为 Terraform 基础设施代码的设计依据。

## 部署选型

| 维度 | 选型 | 说明 |
|------|------|------|
| 数据库 | 阿里云 RDS Serverless（PostgreSQL） | 与 provider 技术方案一致（开发 SQLite / 生产 PostgreSQL，GORM 方言切换）；Serverless 免运维、按需扩缩 |
| 服务计算 | FaaS（函数计算）+ 容器镜像 | Dockerfile 发布 Dockerhub，阿里云镜像拉取或官方仓库直接拉取；服务无需常驻、按调用计费 |
| 存储与网络 | 随计算/数据库一并解决 | VPC 内网互通（RDS ↔ FC） |
| API 网关 | **预留（系统层面统一规划）** | 统一 `api.quanttide.com`，路径按应用名（如 `/qtcloud-pay`）；不在本应用 IaC 范围内，由系统级网关统一接入 |

## 本 IaC 范围

- **系统级共享**（quanttide 体系统一管理，`quanttide-<env>` 命名）：VPC / 交换机 / 安全组、RDS 实例
- **应用级**（`qtcloud-pay-<env>` 命名）：数据库与账号（`qtcloud_pay`）、FC 函数与默认角色
- **不含** API 网关、域名、DNS（系统层面预留）

## 设计动机

- 日志确认：原型已可上线（仅数据更新不便），部署决策完成后即可生产化
- 「计算、存储、网络都解决」——选型目标是最小化运维，Serverless + 托管数据库 + 统一网关
- 统一 API 网关（`api.quanttide.com/<应用名>`）为多应用（qtcloud-pay、后续其他应用）预留统一入口

## 待办

- [x] 按此方案设计 Terraform（IaC）：VPC + RDS Serverless + FC 服务
- [x] state 迁移到 OSS 远端后端（`quanttide-terraform-state`，init 需带 `-backend-config`）
- [x] Dockerfile 已就绪（多阶段构建 + 非 root）；镜像由 deploy-provider workflow 构建发布（`<DockerHub用户>/qtcloud-pay-provider`，为 cli/studio 等预留命名空间）
- [ ] 环境划分（dev / prod）与配置管理（`DB_DRIVER` / `DATABASE_URL` 等，对齐 provider 技术方案）
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-pay`（系统层面预留，另行规划）

## 使用

```sh
terraform init \
  -backend-config="bucket=quanttide-terraform-state" \
  -backend-config="key=qtcloud-pay/terraform.tfstate" \
  -backend-config="region=cn-hangzhou"
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```
