# Terraform 部署 TODO

## 上线前(阻塞项)

- [ ] 检查 `provider/v0.1.0-alpha.1` 发布状态:[Deploy Provider workflow](https://github.com/quanttide/qtcloud-pay/actions/runs/30762142832)(镜像构建 + terraform apply)跑完后验证 FC 服务可用

- [x] 配置阿里云凭证：本地 `~/.aliyun/config.json` + CI GitHub org secrets;RAM 用户授权 `PowerUserAccess` + `AliyunRAMFullAccess`
- [x] state 迁移到 OSS 远端后端（`quanttide-terraform-state`，已创建）
- [x] 镜像由 deploy-provider workflow 构建发布（双通道：Docker Hub + ACR `registry.cn-hangzhou.aliyuncs.com/quanttide/qtcloud-pay-provider`，`provider/*` tag 触发）
- [ ] ACR 命名空间/仓库由 CI 幂等创建（见 docs/dev-guide/iac.md「0.5 ACR 命名空间与镜像仓库」），确认 FC 能从 ACR 拉镜像（registry not reachable 阻塞项）
- [ ] 复制 `terraform.tfvars.example` → `terraform.tfvars` 填写真实值，执行 `terraform apply`
- [ ] 上线验证:通过 `terraform output fc_http_url` 访问账本 API(如健康检查/创建账户)确认全链路可用

## CI 与安全(P1)

- [x] GitHub Actions workflow:terraform init / plan / apply,引用 org secrets(`ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET`)
- [ ] 数据库密码不再明文落入 tfstate(改 FC 配置中心 / 密钥管理注入)
- [ ] 本地/开发凭证收敛为只读（第 2 层生产保护），apply 仅存于 CI

## 后续(P2)

- [ ] **OIDC 联邦升级**:RAM 角色信任 GitHub OIDC,workflow 不再需要 Secret Key,CI 不落任何长期凭证
- [ ] 权限收敛为最小策略集
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-pay`（系统层面预留，另行规划）
- [ ] VPC/RDS 抽离到系统级 IaC（quanttide 体系统一管理，如 quanttide-infra），qtcloud-pay 仅保留 FC 与数据库
