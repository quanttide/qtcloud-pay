# CI 维护指南（qtcloud-pay）

deploy-provider workflow 的架构、凭证体系、踩坑记录与维护清单。基础设施部分见 [iac.md](./iac.md)。

## 概述

[.github/workflows/deploy-provider.yml](../../.github/workflows/deploy-provider.yml) 是唯一部署入口：

- **触发**：推送 `provider/*` tag（如 `provider/v0.1.0-alpha.4`）；**无手动触发**（生产保护，workflow_dispatch 已移除）
- **job 1 `build-image`**：Dockerfile 多阶段构建 → 双通道发布（Docker Hub + ACR）→ 各带版本 tag 与 `latest`
- **job 2 `deploy`**：`terraform init/validate/plan/apply`（OSS 远程状态），挂 GitHub `production` 环境（可配 Required reviewers）
- **并发控制**：`concurrency: deploy-provider`，避免同时 apply 竞争状态

## 凭证与环境变量

### org secrets 清单

| secret | 用途 | 注入方式 |
|--------|------|---------|
| `ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET` | RAM 用户 AccessKey（terraform + aliyun CLI） | terraform：`ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`；aliyun CLI：`ALICLOUD_ACCESS_KEY_ID` / `ALICLOUD_ACCESS_KEY_SECRET` |
| `ALIYUN_ACR_USERNAME` / `ALIYUN_ACR_PASSWORD` / `ALIYUN_ACR_REGISTRY` | ACR 个人版固定凭证（docker login） | `docker/login-action` 的 registry/username/password |
| `DB_PASSWORD` | RDS 数据库密码 | `TF_VAR_db_password` |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD` | Docker Hub 凭证（双通道之一） | `docker/login-action` |

### 关键坑：ALICLOUD_* 新旧变量名

同一份 AccessKey，**两套变量名不互通**：

- **terraform alicloud provider** 读 `ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`（deploy job）
- **aliyun CLI 3.x** 读 `ALICLOUD_ACCESS_KEY_ID` / `ALICLOUD_ACCESS_KEY_SECRET` / `ALICLOUD_REGION_ID`（build job）

写错变量名不会报错，只会"凭证为空"式地失败在远端 API 上，排查成本高。

### 坑：job output 会被 GitHub 脱敏

含 secret 的字符串写进 `$GITHUB_OUTPUT` 会被脱敏置空（如镜像名里拼接 Docker Hub 用户名）。**镜像名等含 secret 的值只写 `$GITHUB_ENV`**，跨 job 传递时在目标 job 内自行重算。

## 镜像双通道发布

- **为什么双通道**：FC 中国区拉不到 Docker Hub（`registry is not reachable`），部署镜像必须走同地域 ACR；Docker Hub 保留为对外分发通道
- **部署镜像固定指向 ACR**：`registry.cn-hangzhou.aliyuncs.com/quanttide/qtcloud-pay-provider:<tag>`（deploy job 的 `TF_VAR_image`）
- **ACR 命名空间/仓库幂等创建**：CI 构建前用 aliyun CLI 实例版 API 确保存在（PUBLIC，FC 免凭证直拉）；terraform provider 的个人版 CR 资源已弃用（v1.276.0 起），不入 IaC
- **tag 解析**：`${GITHUB_REF_NAME#provider/}` 去掉前缀得镜像 tag（`provider/v0.1.0-alpha.4` → `v0.1.0-alpha.4`）

## 踩坑记录

### 1. steps 输出引用错误（镜像版本 tag 从未推送成功）

`Resolve Image Tag` 步骤输出的是 `tag`，但 build 步骤引用了 `steps.image.outputs.name`（不存在）→ 空 tag。**修复**：引用实际输出的 key。教训：引用 `steps.<id>.outputs.<key>` 前先核对 `echo "key=..." >> "$GITHUB_OUTPUT"` 里的 key。

### 2. aliyun action 凭证输入无效

`aliyun/aliyun-cli-action@v1` 的 `access-key-id` / `access-key-secret` / `region-id` 输入**无效**（该 action 只认 `version`，CI annotation 可证实），凭证从未注入。**修复**：换 `aliyun/setup-aliyun-cli-action@v1`（仅装 CLI），凭证经 job env `ALICLOUD_ACCESS_KEY_ID` / `ALICLOUD_ACCESS_KEY_SECRET` / `ALICLOUD_REGION_ID` 注入。教训：第三方 action 的参数以 CI annotation / 实测为准，别信 README 印象。

### 3. ACR API 是实例版（个人版也是）

当前 aliyun CLI（3.4.x）的 `cr` 产品为**实例版 API**：`CreateNamespace` / `CreateRepository` 必填 `--InstanceId`（个人版实例形如 `cri-xxx`），参数名是 `--NamespaceName` / `--RepoName`；老版参数（`--Namespace` / `--NamespaceStatus`）必报 `required parameters not assigned`。**流程**：`ListInstance --InstanceType acr_personal --RegionId cn-hangzhou` 取实例 ID → Get 查存在 → 不存在才创建。

### 4. 错误被吞导致"假成功"

`cmd >/dev/null 2>&1 || echo "already exists"` 把真实失败（参数错误、凭证无效）伪装成幂等跳过，CI 显示 Ensure 步骤成功但仓库从未创建，直到 push 报 `insufficient_scope: authorization failed`。**修复**：`set -euo pipefail` + `GetNamespace`/`GetRepository` 查存在 + 创建失败即 fail。教训：**幂等创建要"查存在→不存在才建"，不要"建失败当已存在"**。

### 5. FC 拉不到 Docker Hub

`registry is not reachable`：FC 中国区访问 Docker Hub 不稳定/不可达，`acceleration_type = "Default"` 无效。**方案**：部署镜像固定 ACR 同地域公开仓库。

### 6. 数据库密码明文落 tfstate

当前最小化实现；生产环境建议改 FC 配置中心/密钥管理注入。

## 调试方法

```sh
# 看失败步骤日志（需要 gh 登录）
gh run view <run-id> --repo quanttide/qtcloud-pay --log-failed

# 看 job/步骤结论
gh run view <run-id> --repo quanttide/qtcloud-pay

# API（匿名可查 run/job 结论，日志下载需 admin）
#   https://api.github.com/repos/quanttide/qtcloud-pay/actions/runs
#   https://api.github.com/repos/quanttide/qtcloud-pay/actions/runs/<id>/jobs
```

经验：先看 `gh run view` 的 job 结论定位失败阶段（build / login / push / terraform），再 `--log-failed` 看具体错误；注意 CI annotation（黄色警告）往往提前暴露配置问题（如 action 参数无效）。

## 维护清单

### 发版

```sh
# 1. 手动补 CHANGELOG（qtcloud-devops 需 LLM_API_KEY 才自动生成，未配置时手动维护 src/provider/CHANGELOG.md）
# 2. 发布（校验 CHANGELOG → 打 tag → GitHub Release）
qtcloud-devops release publish --version provider/vX.Y.Z -y
# 重发已失败版本（删除旧 tag/Release 重建）：加 -f
```

tag 指向的 commit 决定 CI 用的代码——**先推修复再发 tag**；已失败版本建议直接发新版本号（alpha.X+1），语义更干净。

### 改 workflow 后

- [ ] editor YAML 诊断通过（缩进错误会直接破坏 workflow）
- [ ] 新增 secret：org Settings → Secrets → Actions，范围 Public repositories
- [ ] 新变量名确认：terraform 用 `ALICLOUD_ACCESS_KEY/SECRET_KEY`，aliyun CLI 用 `ALICLOUD_ACCESS_KEY_ID/SECRET`
- [ ] 幂等步骤遵守「Get 查存在 → 不存在才建 → 失败即 fail」

### 凭证轮换

- ACR/Docker Hub 是固定凭证（长期 secret），建议定期轮换；后续升级 OIDC 联邦后 CI 不再需要长期 AccessKey
