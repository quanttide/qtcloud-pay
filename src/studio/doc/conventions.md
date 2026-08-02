# 客户端设计约束与实现约定

适用于 studio 全部模块的横切约定。各模块文档见[文档导航](index.md)。

## 与服务端契约一致（联调前提）

客户端所有模型与服务端（[src/provider/docs](../../provider/docs/index.md)）**JSON 字段一一对应**，联调时以服务端为准：

| 约定 | 值 |
|------|-----|
| 字段命名 | JSON snake_case（`customer_id`、`balance_after`），Dart 侧 camelCase 映射 |
| 金额 | 全链路 `int` 整数分，不做浮点；输入经 `AmountField`（元→分）、展示经 `MoneyText`（分→元） |
| 时间 | Go `time.Time` 序列化为 RFC3339（如 `2026-08-03T10:00:00+08:00`），Dart 侧 `DateTime.parse` |
| 枚举 | 与服务端常量一致，见下表 |
| 可选字段 | `omitempty` 字段（`order_id`、`note`、`rate`、`threshold` 等）解析时允许缺失 |
| 幂等键 | 客户端不生成、不存储，只从用户输入透传（见「幂等键」表） |

### 枚举值（与服务端一一对应）

| 枚举 | 值 |
|------|-----|
| 交易类型 `type` | `recharge` 充值 / `consume` 消费 / `issue` 发券 / `redeem` 核销 |
| 优惠券类型 `coupon.type` | `discount` 折扣券 / `full_reduction` 满减券 |
| 适用范围 `scope` | `all` 全场 / `cloud` 云服务 / `course` 课程 / `data` 数据服务 / `product` 指定商品 |
| 券状态 `status` | `issued` 已发放 / `used` 已使用 / `expired` 已过期 |
| 订单状态 `order.status` | `created` 已创建 / `settled` 已结算 |
| 抵扣类型 `deduction.kind` | `coupon` 优惠券 / `voucher` 代金券 / `balance` 余额 |

## 幂等键（联调安全）

| 场景 | 客户端字段 | 服务端唯一约束 | 重复提交行为 |
|------|-----------|----------------|-------------|
| 充值 | `voucher_no` 打款凭证号 | `transaction.idempotency_key`（`recharge:` 前缀） | 查回已有交易，返回成功，不重复入账 |
| 发券 | `batch_no` 发放批次号 | `coupon.batch_no` / `voucher.batch_no` | 不重发 |
| 结算 | `order_id` 商户订单号 | `order.id` | 返回已有订单，不重复结算 |

客户端表单中幂等键字段一律用 `IdempotencyField` 组件（必填 + 唯一性提示）。

## 联调约定

- 开发服务地址：`http://localhost:8080`（由 `services/pay_api.dart` 统一配置）
- 列表响应统一包装：`{"account_id": "...", "transactions"|"coupons"|"vouchers": [...]}`
- 单对象响应：直接返回对象 JSON（`Account`、`Order`、`Statement`）
- 创建类操作返回包装对象：充值 `{account_id}`、发券 `{account_id, batch_no, count}`、账户 201 返回 `Account`
- 错误响应统一 `{"error": "..."}`，客户端按状态码映射（见下）
- 分页：`GET /accounts/{id}/transactions` 支持 `?limit=`（默认 20，上限 100）与 `?offset=`（默认 0）

### 错误状态码映射

| HTTP | 服务端错误 | 客户端表现 |
|------|-----------|-----------|
| 400 | 参数错误：缺字段、金额非正、订单金额非法 | 表单校验提示 |
| 404 | `account: not found` | 账户不存在提示 |
| 409 | `coupon: unavailable` / `voucher: unavailable`（已使用/已过期） | 券不可用提示（对照工作台 §五） |
| 422 | `billing: insufficient balance` 余额不足 | 结算拒绝，订单不落库 |
| 500 | 服务错误 | 通用错误 + 记日志 |

## 客户端分层约定

1. **页面不拼 URL**：端点只在 `services/pay_api.dart` 出现，页面只调方法（端点变更只改一处）
2. **金额只在两处转换**：`AmountField`（元→分）、`MoneyText`（分→元），其余代码一律以分为单位
3. **幂等键必填**：充值/发券/下单表单缺少幂等键时禁止提交
4. **状态管理**：`provider`，`pay_api.dart` 以 ChangeNotifier 暴露数据与操作
5. **客户端不做账务计算**：结算、状态机、余额推导全在服务端，客户端只展示服务端返回结果
6. **错误引导**：异常提示文案对照[工作台 §五 异常处置](../../../../../data/roadmap/studio.md)
