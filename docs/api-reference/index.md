# API 参考 · 账本核心（v0.1.0）

所有端点均以 JSON 交互。

## 金额约定

- **API 传输**：金额为**元**（两位小数数字，如 `99.99`）；`0` 与整数元（`100`）同样合法
- **严格校验**：最多两位小数，三位及以上小数（如 `99.999`）返回 400
- **内部存储**：一律以整数分（`int64`）参与计算，API 边界转换零误差（字符串式解析，不经过浮点舍入）
- **例外**：`POST /reconcile/bank` 的银行流水 CSV 金额为分（`amount_cents`，财务工具格式）

## 端点地图

| 资源 | 文档 | 端点 |
|------|------|------|
| 账户 | [accounts.md](accounts.md) | `POST /accounts`、`GET /accounts/{id}`、`GET /accounts/{id}/transactions`、`POST /accounts/{id}/recharges`、`POST /accounts/{id}/refunds` |
| 优惠券 | [coupons.md](coupons.md) | `POST /accounts/{id}/coupons`、`GET /accounts/{id}/coupons` |
| 代金券 | [vouchers.md](vouchers.md) | `POST /accounts/{id}/vouchers`、`GET /accounts/{id}/vouchers` |
| 订单 | [orders.md](orders.md) | `POST /orders`、`GET /orders/{id}` |
| 对账 | [reconciliation.md](reconciliation.md) | `GET /accounts/{id}/statement`、`GET /reconcile/consistency`、`POST /reconcile/bank` |

## 通用约定

### 错误处理

| 状态码 | 场景 |
|--------|------|
| 400 | 请求体非法、金额非正、金额超过两位小数、缺少必填字段 |
| 404 | 账户/订单不存在 |
| 409 | 账户已存在、券不可用 |
| 422 | 余额不足（结算或退款），整体回滚 |
| 500 | 服务端错误 |

错误响应：

```json
{"error": "invalid request body"}
```

### 幂等键

| 场景 | 幂等键 | 重复提交行为 |
|------|--------|-------------|
| 充值 | `voucher_no` | 返回成功，不重复入账 |
| 退款 | `voucher_no` | 返回成功，不重复退款 |
| 发券 | `batch_no` | 返回成功，不重复发放 |
| 结算 | `order_id` | 返回同一订单，不重复扣款 |

### 分页

`GET .../transactions` 支持 `limit`（默认 20，最大 100）与 `offset`（默认 0），按时间倒序。

### 券状态

`issued`（已发放）/ `used`（已使用）/ `expired`（已过期，查询时惰性流转）。

### 交易类型

| 类型 | 含义 | 影响余额 |
|------|------|---------|
| `recharge` | 充值（对公打款入账） | + |
| `refund` | 退款（多退登记） | − |
| `consume` | 消费（余额支付部分） | − |
| `issue` | 发券（信息性记录） | 无 |
| `redeem` | 核销/抵现（券抵扣部分） | 无 |

## 可靠性保证

- **不丢**：每笔业务动作（充值/发券/扣费/核销/退款）都落在交易账本上
- **不重**：充值/退款/发券/结算均有业务幂等键 + 唯一约束
- **不错**：余额、券状态、交易同事务更新；失败整体回滚
- **可查**：任意交易可追溯（订单号/券关联），账单可导出

## 演进（v0.2.0）

当前版本**不接入支付**——模拟账户模式下，打款登记即完成闭环。v0.2.0 将接入微信 JSAPI 与支付宝网页支付：支付回调验签后自动写入充值交易（幂等键 = 渠道交易号），替代手动登记，**账本模型不变**。
