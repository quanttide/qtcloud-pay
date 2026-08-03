# qtcloud-pay

量潮支付云 — 量潮知识管理体系中的支付云服务平台。

## 概述

qtcloud-pay 是量潮支付领域的云服务平台，提供支付网关、账务处理、对账结算等核心支付能力的云上部署与托管服务。

## 仓库结构

| 路径 | 说明 |
|------|------|
| `src/provider/` | 支付云服务端（Go）：账本核心（M1–M4）+ 支付渠道（微信/支付宝），含自身 README/CONTRIBUTING/ROADMAP |
| `src/cli/` | 账本核心命令行工作台（Rust，命令 `qtcloud-pay`） |
| `src/studio/` | 管理人员图形化工作台（Flutter 桌面应用，Windows/macOS） |
| `docs/` | 领域文档（`user-guide/`、`dev-guide/`、`api-reference/`） |
| `tests/` | 端到端测试目录（当前无 Python 测试文件，pytest 配置为遗留，勿执行 `uv run pytest`） |
| `manifests/` | 部署清单（terraform 等） |
| `.agents/skills/` | 编码 agent 技能（`qtcloud-devops`：命令行开发运维工作流） |

## 测试

```sh
# provider 服务端：Go 单元 + 集成测试
cd src/provider && make test

# cli：Rust 测试
cd src/cli && cargo test

# studio：Flutter 测试
cd src/studio && flutter test
```

## 文档导航

- [CONTRIBUTING.md](CONTRIBUTING.md) — 贡献指南（提交规范、契约纪律、scope 开发命令）
- [AGENTS.md](AGENTS.md) — 编码 agent 工作索引与纪律
- `src/provider/` — [README](src/provider/README.md) / [CONTRIBUTING](src/provider/CONTRIBUTING.md) / [ROADMAP](src/provider/ROADMAP.md) / [设计文档](src/provider/docs/index.md)
- `docs/user-guide/` — 用户指南（[index](docs/user-guide/index.md)、journey、billing）
- `docs/dev-guide/` — 开发指南（ci、iac）

## 许可

Apache 2.0
