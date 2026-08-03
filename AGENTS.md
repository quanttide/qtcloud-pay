# AGENTS（qtcloud-pay）

面向在仓库内工作的编码 agent 的指令。**动手前先读「关键文件」一节**；涉及工具库契约时严格遵守「harness 工作纪律」。本仓库是 `quanttide-pay` 的子模块，上级纪律见主仓库 [AGENTS.md](../../AGENTS.md)。

## 仓库是什么

量潮支付云服务平台：`src/provider`（Go 服务端：账本核心 + 支付渠道）、`src/cli`（Rust 命令行工作台）、`src/studio`（Flutter 管理工作台）、`docs/`（领域文档）、`manifests/`（部署清单）。

## 关键文件（按优先级阅读）

| 文件 | 作用 | 何时必读 |
|------|------|----------|
| `README.md` | 仓库结构、scope 导航、测试命令 | 每次工作前 |
| `CONTRIBUTING.md` | 提交规范、工具库契约纪律、发布约定 | 每次提交前 |
| `.agents/skills/qtcloud-devops/SKILL.md` | 命令行开发运维工作流（构建/测试/发布/启动） | 涉及构建测试发布 |
| `src/provider/README.md` | provider 结构、API、渠道、测试分层 | 改 provider 前 |
| `src/provider/CONTRIBUTING.md` | provider 设计约束（账本写入唯一入口、同事务更新等） | 改 provider 前 |
| `src/provider/ROADMAP.md` | 缺陷清单（F1–F8）、v0.2.0 计划（T1–T5）、待定决策 | 改 provider 前 |
| `src/provider/docs/conventions.md` | 横切设计约束与实现约定（全模块公约数） | 改 provider 模块前 |
| `src/provider/docs/index.md` + 各模块 `docs/<module>.md` | 模块设计文档（account/transaction/coupon/voucher/billing/order/reconciliation/channel） | 改对应模块前 |
| `src/cli/README.md` | CLI 用法（以 `--help` 为准）、退出码约定 | 改 cli 前 |
| `src/studio/README.md`、`src/studio/doc/index.md` | 工作台模块划分（客户端只做展示，不承载账务逻辑） | 改 studio 前 |
| `../../packages/quanttide-pay-toolkit/tests/fixtures/*.json` | **契约唯一权威**（工具库 fixtures） | 涉及契约语义时 |
| `../../packages/quanttide-pay-toolkit/packages/go/STATUS.md` | 工具库交接状态与待办 | 涉及工具库改动时 |

## harness 工作纪律（最高优先级）

本仓库是工具库契约的**消费端**，契约权威在工具库 `tests/fixtures/`（JSON）+ `contracttest`（Go runner）：

1. **禁止端侧发明语义**：状态（`pkg/status`）、幂等键（`pkg/idempotency`）、计费计算（`pkg/billing`）、交易类型（`pkg/ledger`）等契约一律引用工具库，不自行定义等价常量/算法；契约演进由 fixtures 驱动两端同步
2. **未知码必须报错**：渠道原始码解析遇未知值返回错误，不用 UNKNOWN 兜底（新渠道状态暴露而不是被掩盖）
3. **金额整数分**：全链路 int64 分，禁止浮点；API 边界元/分转换集中在 transport 层
4. **主干边界**：纯逻辑（值对象/枚举/纯函数）提炼进工具库；本仓库只留实体、gorm、事务编排、渠道适配
5. **不动工具库改引用**：发现契约缺失/不符，先回工具库按 fixtures 流程改，再回本仓库更新引用（`go.mod` replace 或 go.mod 依赖版本）

## 工作纪律

- 命令工作流（构建/测试/发布/启动）以 `.agents/skills/qtcloud-devops/SKILL.md` 为准；**`tests/` 无 Python 测试，勿执行 `uv run pytest`**
- 密钥只经环境变量注入，不写入代码、文档或提交历史
- 渠道适配器、transport 的改动保持与 `docs/conventions.md` 一致（账本写入唯一入口、同事务更新、存储双引擎、渠道独立演进、无后台任务）
- provider 缺陷修复优先对照 `src/provider/ROADMAP.md` 的 F1–F8 清单

## 提交纪律

- 本仓库独立提交与推送；内容变更后在主仓库（`quanttide-pay`）更新子模块指针
- Conventional Commits，中文描述（如 `refactor(billing): 纯计算提炼至工具库 pkg/billing`）
- 提交前核对：相关 ROADMAP/CHANGELOG 是否同步、`make test`（或对应 scope 测试）是否通过
