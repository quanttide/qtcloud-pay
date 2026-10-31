# CHANGELOG

## [Unreleased]

### Added

- 发券页新增「发放规则」只读页签（`src/studio`）：调 `GET /admin/voucher-pricing-rules` 展示发放渠道、触发条件、面额与面额规则，供监管角色（viewer）查看；新增模型 `PricingRuleSet`/`IssuanceChannel` 与解析单元测试
- 初始化 Flutter 客户端 `qtcloud_pay_studio`（`src/studio`）
- Python 端到端测试（`tests/`）：pytest fixture 编译并启动 provider 真实二进制（临时 SQLite 库），覆盖 tests.md 全部 25 个业务场景 TC + 3 冒烟用例；`api.py` 提供纯标准库 HTTP 客户端与账本断言

### Changed

- 代金券计价规则集的两个 GET 端点（列表/详情）不再校验 `X-Admin-Token`：只读由权限中间件按 `rule:read` 把关（viewer 可看发放规则），PUT 录入仍需 admin token

## [0.0.1] - 2026-07-11

阶段晋级至 v0.0.1
