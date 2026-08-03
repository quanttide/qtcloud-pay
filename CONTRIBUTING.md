# 贡献指南 · qtcloud-pay

qtcloud-pay 支付云服务平台（独立仓库，作为 `quanttide-pay` 的子模块挂载于 `apps/qtcloud-pay`）。

## 仓库结构须知

| scope | 语言 | 说明 | 规范文档 |
|-------|------|------|----------|
| `src/provider` | Go | 支付云服务端：账本核心 + 支付渠道 | [README](src/provider/README.md)、[CONTRIBUTING](src/provider/CONTRIBUTING.md)、[ROADMAP](src/provider/ROADMAP.md)、[docs](src/provider/docs/index.md) |
| `src/cli` | Rust | 账本核心命令行工作台 | [README](src/cli/README.md) |
| `src/studio` | Flutter | 管理人员图形化工作台 | [README](src/studio/README.md)、[doc](src/studio/doc/index.md) |
| `docs/` | Markdown | 领域文档（user-guide / dev-guide / api-reference） | — |
| `manifests/` | Terraform 等 | 部署清单 | [docs/dev-guide/iac.md](docs/dev-guide/iac.md) |

## 提交规范

Conventional Commits，描述用中文，参照历史：

```
refactor: transport 公共件/中间件/券与订单状态提炼至工具库
refactor(billing): 纯计算提炼至工具库 pkg/billing，本模块改别名转发
```

- 类型：`feat` / `fix` / `refactor` / `docs` / `test` / `chore`
- 作用域：`billing` / `idempotency` / `transaction` / `channel` 等模块名，或 `cli` / `studio` / `docs`
- 一次提交一个逻辑变更；文档与代码同批提交

## 工具库契约纪律（重点）

本仓库大量能力来自工具库 `quanttide-pay-toolkit`（`pkg/status`、`pkg/idempotency`、`pkg/billing`、`pkg/ledger`、`pkg/money`、`pkg/httpapi`、`pkg/middleware`）：

1. **禁止端侧发明语义**：状态、幂等键、计费规则等契约语义一律引用工具库，不得在 provider 内自行定义等价常量/算法；工具库 fixtures 是契约唯一权威
2. **金额整数分**：全链路 int64 分存储，禁止浮点；API 层仅在边界做元/分转换
3. **渠道原始码解析**：未知码必须报错，不用 UNKNOWN 兜底（工具库 `pkg/status` 已实施）
4. **主干边界**：纯逻辑提炼进工具库（provider 只留实体、gorm 存储、事务编排、渠道适配）

## 各 scope 开发命令

```sh
# provider（Go）：单元 + 集成测试
cd src/provider && make test && make vet

# cli（Rust）
cd src/cli && make build && cargo test

# studio（Flutter）
cd src/studio && flutter pub get && flutter test
```

注意：`tests/` 目录无 Python 测试文件（pytest 配置为遗留），**不要执行 `uv run pytest` 作为验证步骤**。

## 发布

- 版本标签约定：根项目 `vX.Y.Z`（记录 `pyproject.toml` + `CHANGELOG.md`）、provider `provider/vX.Y.Z`（仅 `CHANGELOG.md`）
- 使用 `qtcloud-devops release publish --version <tag> -y` 发布（详见 [.agents/skills/qtcloud-devops/SKILL.md](.agents/skills/qtcloud-devops/SKILL.md)）

## 不做的事

- 不在本仓库直接实现工具库已有契约（先提炼到工具库，再回来改引用）
- 密钥不写入代码、文档或提交历史（一律环境变量注入）
- 不新增后台定时任务（券过期用惰性流转，对账按需调用）
