# channel 支付渠道（现有，M5）

包：`internal/channel`（transport / service / adapters / model + wechat/ + alipay/）

## 职责

微信 JSAPI（公众号/小程序）、支付宝网页支付（PC）渠道；下单/查询/退款/通知解析。是后接的可替换渠道层。

## 现状

| 能力 | 微信 JSAPI | 支付宝网页支付 |
|------|-----------|---------------|
| 下单 | `JSAPIPay` → prepay_id + 前端调起参数 | `PagePay` → HTML 表单 / `WapPay` → HTML 表单 |
| 查询 | `QueryOrder` / `QueryOrderByOutTradeNo` | `QueryOrder` |
| 退款 | `Refund` | `Refund` |
| 通知解析 | `ParseNotify`（AES-GCM 解密 + 验签） | `VerifyNotify`（RSA2 验签） |

v0.1.0 不做扩展，保持独立（关键设计约束 4）。**刻意平行**：渠道不写 `order` 表、支付成功不入账；`-channel` flag 默认为空，生产 FC 部署即纯账本 API（terraform 无渠道环境变量）。「结算即支付」仅限余额/券抵扣，外部支付闭环（回调→入账）推迟到 v0.2.0。

## 已知缺陷（2026-08-03 评审，见 ROADMAP）

- F2：渠道层金额 float64 元→分 `int(x*100)` 截断精度错误，待改 int64 分
- F3：通知解析已实现但无回调路由，支付成功无法入账
- F4/F5/F6/F7：退款参数、退款状态、错误处理、证书序列号——详见 ROADMAP

## v0.2.0 接入计划

- 新增支付回调 handler：验签（`ParseNotify` / `VerifyNotify`）→ `transaction.Append`（type=recharge，幂等键=渠道交易号）→ 自动入账，替代手动登记
- 生产验证：通道能力已实现但未经过完整生产验证，接入时逐步验证
- 演化路径落地：手动登记 → 系统化记录 → 自动化执行。**模型不变，变的只是交易来源**

## 测试

现有测试覆盖率 >95%，接入账本后补充回调 → 入账的集成测试。
