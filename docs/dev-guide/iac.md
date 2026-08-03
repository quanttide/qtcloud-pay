# 基础设施即代码（IaC）开发指南

## 概述

qtcloud-pay 的部署由 [Terraform](../../manifests/terraform/) 管理,覆盖:

- **系统级共享**(quanttide 体系统一管理,`quanttide-<env>` 前缀):VPC / 交换机 / 安全组、RDS 实例
- **应用级**(`qtcloud-pay-<env>` 前缀):数据库与账号(`qtcloud_pay`)、FC 函数与默认角色

选型依据支付工程日志的部署决策,详见 [manifests/terraform/README.md](../../manifests/terraform/README.md)。**API 网关(`api.quanttide.com`)为系统层面预留,不在本 IaC 范围内。**

## 命名规则

资源命名按「层级 + 环境」分层:前缀决定归属,环境后缀决定隔离。

### 前缀分层

| 层级 | 前缀 | 资源 | 例子(prod) |
|------|------|------|-----------|
| 系统级(quanttide 体系统一管理) | `quanttide-<env>` | VPC / 交换机 / 安全组 / RDS 实例 | `quanttide-prod` |
| 应用级 | `<app>-<env>` | FC 函数、RAM 角色 | `qtcloud-pay-prod`、`qtcloud-pay-prod-fc` |
| 应用级(数据库) | `<app>`(下划线) | RDS 数据库与账号 | `qtcloud_pay` |

- **系统级资源是共享的**(VPC/RDS 供各应用共用),未来抽离到系统级 IaC 统一管理;应用级资源归属 qtcloud-pay 自身
- RAM 角色加 `-fc` 后缀(`qtcloud-pay-prod-fc`),按职责区分
- 多环境(dev/staging/prod)通过 `environment` 变量隔离命名,**每个环境独立一套系统级资源**(独立 VPC / RDS 实例,`quanttide-<env>`),数据库名因此无需环境后缀(各环境互不冲突)

### 其他命名约定

| 对象 | 规则 | 例子 |
|------|------|------|
| Docker 镜像 | `<用户名>/qtcloud-pay-<组件>` | `qtcloud-pay-provider`(为 cli/studio 预留 `qtcloud-pay-cli` / `qtcloud-pay-studio`) |
| OSS 状态桶 | `quanttide-terraform-state`(系统级共享) | — |
| state key | `<app>/terraform.tfstate`;多环境按环境分 key | `qtcloud-pay/terraform.tfstate` |
| GitHub secrets | 全大写蛇形,语义化 | `ALIYUN_ACCESS_KEY_ID` / `DB_PASSWORD` / `DOCKERHUB_USERNAME` |
| 版本 tag | `<scope>/vX.Y.Z`(qtcloud-devops release) | `provider/v0.1.0-alpha.1` |
| API 域名 | `api.quanttide.com/<app>`(系统级预留) | `api.quanttide.com/qtcloud-pay` |

## 环境准备

### 1. 阿里云凭证（机器级配置）

Terraform 通过 `~/.aliyun/config.json` 读取凭证(与 aliyun CLI 共用同一份配置):

```json
{
  "current": "default",
  "profiles": [
    {
      "name": "default",
      "mode": "AK",
      "access_key_id": "LTAI...",
      "access_key_secret": "...",
      "region_id": "cn-hangzhou",
      "output_format": "json"
    }
  ]
}
```

```sh
chmod 600 ~/.aliyun/config.json
```

**关键原理(踩坑记录)**:alicloud provider 的 `shared_credentials_file` 默认指向 `~/.aliyun/config.json`,但源码(`getConfigFromProfile`)中**只有显式设置了 `profile` 时才会读取该文件**——`if profile 未设置 → return nil`。因此光有文件不够,必须让 `profile` 非空,在 `~/.bashrc` 中追加:

```sh
export ALICLOUD_PROFILE=default
```

这样**本机任意目录**运行 terraform 都会自动读取该文件。多环境时在 `config.json` 中增加 profile,切换 `export ALICLOUD_PROFILE=<name>` 即可。

### 2. RAM 用户与权限

- 创建 RAM 用户,勾选 **OpenAPI 调用访问**,生成 AccessKey(Secret 只显示一次)
- 授权:**`PowerUserAccess` + `AliyunRAMFullAccess`**(所有资源全权限 + RAM 管理;后者是 IaC 创建 FC 角色/策略所需)
- 开发机图省事可直接用 `AdministratorAccess`
- ⚠️ 不要用主账号 AccessKey;凭证文件与密钥一律不入库

### 3. CI 凭证（GitHub Actions）

- AccessKey 配置在 **GitHub org secrets**(`ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET`),org 内仓库可用
- workflow 中通过 `${{ secrets.ALIYUN_ACCESS_KEY_ID }}` 注入 provider 环境变量(`ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`)
- 后续升级:**OIDC 联邦**(RAM 角色信任 GitHub OIDC),CI 不再需要长期 Secret Key

## 使用

```sh
cd manifests/terraform

# 1. 初始化（拉取 provider + 连接 OSS 远程状态；需先创建状态桶）
terraform init \
  -backend-config="bucket=quanttide-terraform-state" \
  -backend-config="key=qtcloud-pay/terraform.tfstate" \
  -backend-config="region=cn-hangzhou"

# 2. 填写参数
cp terraform.tfvars.example terraform.tfvars   # 修改 db_password / image 等

# 3. 预览与执行
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars

# 4. 查看输出（FC HTTP 直连地址等）
terraform output
```

## CI 部署

推送 `provider/*` tag（如 `provider/v0.1.0`）触发 [.github/workflows/deploy-provider.yml](../../.github/workflows/deploy-provider.yml) 自动 `terraform apply`。所需 org secrets：`ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET` / `DB_PASSWORD` / `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD`。

常用变量见 [variables.tf](../../manifests/terraform/variables.tf);待办与排期见 [TODO.md](../../manifests/terraform/TODO.md)。

## 注意事项

- **数据库密码会明文落入 tfstate**:当前为最小化实现,后续应改用密钥管理/配置中心注入
- **状态存储**:已迁移到 OSS 远端后端(`quanttide-terraform-state`),初始化命令见上;多人协作无需再担心状态丢失
- **镜像发布**:`image` 变量指向的容器镜像需已发布(Docker Hub 公开仓库或 ACR),FC 才能拉取
- **环境划分**:默认 `prod`(`TF_VAR_environment`);RDS 系列固定 `serverless_basic`(单节点)
