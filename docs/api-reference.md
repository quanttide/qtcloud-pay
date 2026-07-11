# API 参考

> 本文档以集成测试为事实源自动对齐。运行 `uv run pytest tests/ -v` 可查看最新端点清单。

---

## POST /pay

创建支付订单，返回支付链接。

### 请求

**Content-Type:** `application/json`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `order_id` | string | 是 | 商户订单号 |
| `amount` | number | 是 | 金额（元） |
| `subject` | string | 是 | 商品标题 |
| `notify_url` | string | 否 | 异步通知地址 |
| `return_url` | string | 否 | 同步跳转地址 |
| `metadata` | object | 否 | 额外参数（如微信 `openid`） |

示例：

```json
{
  "order_id": "ORD001",
  "amount": 99.99,
  "subject": "测试课程"
}
```

### 成功响应（200）

```json
{
  "trade_id": "mock_tx_ORD001",
  "pay_url": "https://pay.example.com/ORD001",
  "raw_response": null
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `trade_id` | string | 支付平台交易号 |
| `pay_url` | string | 支付链接 |
| `raw_response` | any | 支付平台原始响应 |

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 400 | 请求体不是合法 JSON |
| 405 | 使用了非 POST 方法（如 GET /pay） |
| 500 | Provider 返回错误（如服务不可用） |

---

## GET /query/{order_id}

查询订单支付状态。

### 请求

**Path Parameters:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `order_id` | string | 是 | 商户订单号 |

### 成功响应（200）

```json
{
  "trade_id": "mock_tx_ORD001",
  "order_id": "ORD001",
  "status": "SUCCESS",
  "amount": 99.99,
  "paid_at": "2025-07-11T10:00:00+08:00"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `trade_id` | string | 支付平台交易号 |
| `order_id` | string | 商户订单号 |
| `status` | string | 订单状态（SUCCESS / PENDING / CLOSED / UNKNOWN） |
| `amount` | number | 实付金额（元） |
| `paid_at` | string | 支付时间 |

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 400 | `order_id` 为空或仅含空白字符 |
| 405 | 使用了非 GET 方法（如 POST /query/xxx） |
| 500 | 查询失败（如订单不存在） |

---

## POST /refund

申请退款。

### 请求

**Content-Type:** `application/json`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `order_id` | string | 是 | 商户订单号 |
| `trade_id` | string | 否 | 支付平台交易号 |
| `refund_amount` | number | 是 | 退款金额（元） |
| `reason` | string | 否 | 退款原因 |

示例：

```json
{
  "order_id": "ORD001",
  "refund_amount": 50.00,
  "reason": "部分退款"
}
```

### 成功响应（200）

```json
{
  "refund_id": "mock_rf_ORD001",
  "status": "SUCCESS"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `refund_id` | string | 退款单号 |
| `status` | string | 退款状态（SUCCESS / PROCESSING / CLOSED） |

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 400 | 请求体为空或不是合法 JSON |
| 405 | 使用了非 POST 方法（如 GET /refund） |
| 500 | Provider 返回错误（如退款被拒绝） |
