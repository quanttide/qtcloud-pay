# provider ROADMAP

按版本组织，只列直接需要落实到代码的事项。权威路线图见[域级路线图](../../../../data/roadmap/provider.md)。

## v0.1.0 账本核心（已交付 2026-08-03）

- 账户与账本、优惠券/代金券、订单与结算、对账与可查（M1–M4）
- **按量/按次扣费**：`consume`（课堂按学习扣费、云按量、数据按交付进度）
- **多退少补**：退款登记 `POST /accounts/{id}/refunds`（`refund:{voucher_no}` 幂等，余额不足 422）+ 再充值

## 缺陷修复（2026-08-03 代码评审，未开始）

| # | 优先级 | 问题 | 落点 | 状态 |
|---|--------|------|------|------|
| F1 | P0 | 账本 API 零认证 + FC 触发器 `anonymous`：公网任意人可充值/退款/发券，**生产阻塞**（网关未接入前必须有应用层鉴权） | `internal/middleware` 新增鉴权中间件，`app.BuildMux` 挂载 | 未开始 |
| F2 | P0 | 微信金额 float64 元→分 `int(x*100)` 截断精度错误（0.29→28 分） | `internal/channel` 模型改 int64 分（对齐 `money.Cents`），删 `*100` 转换与 `parseAmount` | 未开始 |
| F3 | P1 | 支付通知无回调路由：`ParseNotify` 已实现未挂载，支付成功无法入账 | `channel.RegisterRoutes` 补 `POST /notify` + 回调验签（与 T5 联动） | 未开始 |
| F4 | P1 | 微信退款参数错误：`TotalAmount` 填了退款金额（应原单总额）；`OutRefundNo=OrderID+"-REFUND"` 同一订单只能退一次 | `internal/channel/adapters.go` Refund：TotalAmount 传原单总额、OutRefundNo 独立生成 | 未开始 |
| F5 | P1 | 支付宝退款固定返回 SUCCESS，无退款状态查询，对账不可信 | `internal/channel/alipay` 补退款查询（alipay.trade.fastpay.refund.query） | 未开始 |
| F6 | P1 | 渠道 API 错误一律 500（参数错误应 400）；`openid` 缺失时静默传空串 | `internal/channel/transport.go` 参数校验 400、openid 必填校验 | 未开始 |
| F7 | P1 | 微信证书序列号 `Text(16)` 小写且去前导零，未按微信大写十六进制规范（待联调验证，若失败须 ToUpper+补零） | `internal/channel/wechat` parseCertSerial | 未开始 |
| F8 | P2 | DB 密码明文落 tfstate（已记录于部署语境，未解决） | `manifests/terraform` 环境变量改密钥管理注入 + `app.go` 配置读取 | 未开始 |

P2 小项（随迭代清理）：`channel.Server` 死代码（NewServer/Start/Close/Shutdown/SetHandler 无人调用）；`Logging` 无 recover/request ID；AutoMigrate 生产 schema 无版本化；渠道下单/查询未落 `order` 表（与 T5 一并设计）。

## v0.2.0 引擎解耦与支付通道（计划）

| # | 优先级 | 任务 | 落点 | 状态 |
|---|--------|------|------|------|
| T1 | P1 | 方案下发：商品目录 | 新模型 `Product`（id、scope、定价）与下发/管理 API | 未开始 |
| T2 | P1 | 方案下发：计费规则管理 API | `BillingRule` 表已有（priority/condition），补 CRUD 端点 | 未开始 |
| T3 | P1 | 方案下发：券策略参数化 | `billing.Calculate` 硬编码的力度选择/抵扣顺序改为规则驱动 | 未开始，依赖 T1/T2 与 P0 契约确认 |
| T4 | P1 | 账本上报：结算报表 | `reconciliation` 扩展：按账期/账户的结算报表端点（回给商务云） | 未开始 |
| T5 | P2 | 支付回调自动入账 | `channel` 回调（`ParseNotify`/`VerifyNotify`）→ `transaction.Append`（type=recharge，幂等键=渠道交易号） | 未开始 |

## 待定决策（影响 v0.2.0 实现方式，等 P0 契约确认）

- **层边界仲裁**：满减力度选择、代金券不找零归商务云策略还是支付云执行语义——决定 T3 是「配置化」还是「保持硬编码、接口透传」
- **方案契约范围**：商品目录与适用范围的关系（scope 归商品目录还是独立维度）——决定 T1 的模型设计

## 2026-09-02 加急补充：实训基地代金券计价事实

- 已新增 `voucher.PricingRuleSet` 规则集快照与 `/admin/voucher-pricing-rules` 管理 API，用于录入 qtclass 支付工程档案中的发行渠道、核销定价和开放问题。
- 管理 API 按现有 `ADMIN_TOKEN` + `X-Admin-Token` fail-closed 保护；F1 全账本 API 鉴权仍是生产阻塞遗留项。
- `BillingRule` 仍只表达抵扣顺序；一对一咨询“服务者职级档位”和超额申请“流程配额”先保存在规则集 payload，后续若进入自动计费需扩展规则引擎行模型。
