# 贡献指南 · qtcloud-pay CLI

`src/cli`（Rust）——账本核心命令行工作台，命令 `qtcloud-pay`。上级纪律见仓库根 [CONTRIBUTING.md](../../CONTRIBUTING.md)。

## 定位

- 对接服务端账本 API（`src/provider` M1–M4），只做**客户端封装**：命令解析、HTTP 调用、输出格式化；不承载账务逻辑
- 服务端地址：`--server <URL>` / `QTPAY_SERVER_URL` / `~/.config/qtcloud-pay/config.toml`，缺省 `http://localhost:8080`
- 退出码约定：0 成功 / 1 业务错误 / 2 用法错误 / 3 网络与服务端错误

## 提交规范

Conventional Commits，中文描述，作用域 `cli`：

```
feat(cli): 新增账户余额查询子命令
```

## 开发命令

```sh
cargo build --release   # 或 make build
cargo test              # 或 make test
cargo fmt --check
```

## 关键纪律

1. **CLI 用法以 `--help` 输出为准**：命令树会演进，不依赖硬编码命令清单；新增子命令必须提供清晰的 `--help`
2. **不重复实现服务端逻辑**：幂等键、金额换算、状态语义等一律走服务端 API；客户端不自行推导账务规则
3. **金额展示**：CLI 按 API 契约展示金额（元/分），不做浮点累积计算
4. 密钥与敏感信息不经 CLI 参数传递（环境变量/配置文件）

## 测试

```sh
cargo test
```

覆盖：命令解析、参数校验、HTTP 错误映射（业务错误/网络错误 → 退出码）、里程碑验收子命令。
