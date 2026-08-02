---
name: qtcloud-devops
description: qtcloud-pay 的命令行开发运维工作流。当用户要求构建、测试、发布、部署 qtcloud-pay，或用 qtcloud-pay / qtcloud-devops 命令行工具操作账本核心或发布版本时使用。覆盖 go test / cargo test / make build 构建测试、qtcloud-pay CLI 用法（以 --help 为准）、qtcloud-devops release 发布流程。
---

# qtcloud-devops · qtcloud-pay 命令行开发运维

本 skill 只涉及命令行相关工作流。**CLI 用法一律通过 `--help` 学习，不依赖硬编码的命令清单**——命令树会演进，以工具自身输出为准。

## 命令行工具

| 工具 | 位置 | 用途 |
|------|------|------|
| `qtcloud-pay` | `src/cli`（Rust，`make build` 或 `cargo build --release` 构建） | 账本核心命令行工作台：账户/充值/发券/订单/对账/里程碑验收 |
| `qtcloud-devops` | 外部 CLI，非本仓库内置 | 发布流程：CHANGELOG 追加、pyproject 版本更新（仅根项目）、git tag 推送、GitHub Release |
| `make` | `src/provider/Makefile`（build/run/test/vet/lint/clean）、`src/cli/Makefile`（build/run/test/check） | 构建与测试入口 |
| `go` / `cargo` / `flutter` | — | provider / cli / studio 各自的测试与构建 |

## 学习 CLI 用法

动手前先问工具本身：

```bash
qtcloud-pay --help                # 总览子命令
qtcloud-pay <子命令> --help       # 具体命令的必填参数、幂等键选项
qtcloud-pay completions bash      # 生成 shell 补全（可选）
```

`qtcloud-devops` 同理：`qtcloud-devops --help` / `qtcloud-devops release --help`。

## 构建与测试

```bash
# provider（Go）
cd src/provider
make build        # 产物 bin/provider-server
make test         # go test ./...

# cli（Rust）
cd src/cli
make build        # 或 cargo build --release，命令为 qtcloud-pay
cargo test

# studio（Flutter）
cd src/studio
flutter pub get
flutter test
```

注意：`tests/` 无 Python 测试文件（pytest 配置残留在 pyproject.toml），不要执行 `uv run pytest` 作为验证步骤。

## 服务启动

```bash
cd src/provider
./bin/provider-server -addr :8080            # 仅账本 API
./bin/provider-server -addr :8080 -channel wechat   # 挂载支付渠道
```

配置经环境变量：`DB_DRIVER`（postgres/缺省 sqlite）、`DATABASE_URL`、`DB_SQLITE_DSN`、`WECHAT_*` / `ALIPAY_*`。密钥只经环境变量传入，不写入代码或文档。

CLI 默认连 `http://localhost:8080`，可用 `--server <URL>` / `QTPAY_SERVER_URL` / `~/.config/qtcloud-pay/config.toml` 覆盖。退出码约定：0 成功 / 1 业务错误 / 2 用法错误 / 3 网络或服务端错误。

## 发布流程

使用 `qtcloud-devops release` 命令：

```bash
# 发布 provider 模块
qtcloud-devops release publish --version provider/v0.0.1 -y

# 发布根项目
qtcloud-devops release publish --version v0.0.1 -y
```

版本标签约定：根项目 `vX.Y.Z`（记录 `pyproject.toml` + CHANGELOG）、provider `provider/vX.Y.Z`（仅 CHANGELOG）。Go 模块版本由 git tag 决定，`go.mod` 只声明模块路径。

如果 `qtcloud-devops` 命令不可用，提示用户安装/配置该 CLI，不要手动伪造发布步骤。

## 环境要求

- Go >= 1.26、Rust + cargo、Flutter（studio）、Python >= 3.12（仅 pyproject 声明）
