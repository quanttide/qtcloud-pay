# API 参考 · 账本核心（v0.1.0）

所有端点均以 JSON 交互，金额一律**整数分**（100 元 = `10000`）。

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
| 400 | 请求体非法、金额非正、缺少必填字段 |
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
