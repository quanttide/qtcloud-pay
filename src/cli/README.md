# qtcloud-pay-cli

量潮支付账本核心命令行工作台 — 命令 `qtcloud-pay`。

设计文档：[data/roadmap/cli.md](../../../../data/roadmap/cli.md)

## 构建

```sh
cargo build --release
```

## 使用

```sh
qtcloud-pay --help
qtcloud-pay status
qtcloud-pay accounts create --name "示例客户"
qtcloud-pay recharges create <账户> --amount 100.00 --receipt-no 20260802-001
qtcloud-pay reconcile <账户>... --bank bank.csv
```

- 服务端地址：`--server <URL>` / `QTPAY_SERVER_URL` 环境变量 / `~/.config/qtcloud-pay/config.toml`，缺省 `http://localhost:8080`
- 依赖账本核心 API（[provider.md](../../../../data/roadmap/provider.md) M1–M4 就绪后可用）
- 退出码：0 成功 / 1 业务错误 / 2 用法错误 / 3 网络与服务端错误
- 里程碑验收：`qtcloud-pay milestone verify M1` 自动执行 M1 验收（在测试环境执行）

## 测试

```sh
cargo test
```

## 许可

Apache 2.0
