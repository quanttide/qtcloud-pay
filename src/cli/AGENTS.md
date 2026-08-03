# AGENTS（qtcloud-pay · src/cli）

面向在 CLI（Rust）内工作的编码 agent 的指令。上级纪律见仓库根 [AGENTS.md](../../../AGENTS.md)。

## 本 scope 是什么

账本核心命令行工作台（Rust，命令 `qtcloud-pay`）：账户/充值/发券/订单/对账/里程碑验收的客户端封装。**只做展示与调用，不承载账务逻辑**。

## 关键文件

| 文件 | 作用 |
|------|------|
| `README.md` | 构建、使用示例、退出码约定 |
| `CONTRIBUTING.md` | 本 scope 贡献规范 |
| `Cargo.toml` / `Makefile` | 依赖与构建测试入口 |
| 服务端设计 `../provider/docs/index.md` | 账本 API 契约（M1–M4） |
| 域级设计 `../../../data/roadmap/cli.md`（主仓库） | 需求来源 |

## 纪律

1. **CLI 用法以 `--help` 为准**，不硬编码命令清单；新增子命令补 `--help`
2. **不重复实现服务端逻辑**：幂等键/金额/状态语义走服务端 API
3. 退出码：0 成功 / 1 业务错误 / 2 用法错误 / 3 网络与服务端错误
4. 服务端地址解析顺序：`--server` > `QTPAY_SERVER_URL` > `~/.config/qtcloud-pay/config.toml` > 缺省 `http://localhost:8080`

## 验证

```bash
cargo test
```

提交前核对：测试通过、`--help` 输出与实际子命令一致、CHANGELOG 是否需更新。
