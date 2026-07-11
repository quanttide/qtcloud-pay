# 用户指南 · 接入 qtcloud-pay

## 概述

qtcloud-pay 提供统一的支付 API 网关，屏蔽微信、支付宝等支付提供商的差异，提供一致的支付、查询、退款体验。

## 前置条件

- 获取服务地址（如 `https://pay.quanttide.com`）
- 商户已在后台配置有效的支付提供商凭证

## 支付接入

### 1. 发起支付

向支付端点发起 POST 请求创建订单：

```http
POST /pay
Content-Type: application/json

{
  "order_id": "ORD20250711001",
  "amount": 99.99,
  "subject": "《Python 入门课程》"
}
```

成功响应：

```json
{
  "trade_id": "mock_tx_ORD20250711001",
  "pay_url": "https://pay.example.com/ORD20250711001",
  "raw_response": null
}
```

| 字段 | 说明 |
|------|------|
| `trade_id` | 支付平台交易号，用于后续查询和退款 |
| `pay_url` | 支付链接，前端跳转或生成支付二维码 |

#### 带参数的支付请求

微信 JSAPI 支付需传入用户 `openid`：

```json
{
  "order_id": "ORD20250711001",
  "amount": 99.99,
  "subject": "《Python 入门课程》",
  "metadata": {
    "openid": "o-wx-openid-123"
  }
}
```

### 2. 查询订单状态

```http
GET /query/ORD20250711001
```

成功响应：

```json
{
  "trade_id": "mock_tx_ORD20250711001",
  "order_id": "ORD20250711001",
  "status": "SUCCESS",
  "amount": 99.99,
  "paid_at": "2025-07-11T10:00:00+08:00"
}
```

| 字段 | 说明 |
|------|------|
| `status` | `SUCCESS` 支付成功 / `PENDING` 待支付 / `CLOSED` 已关闭 / `UNKNOWN` 未知 |

### 3. 申请退款

```http
POST /refund
Content-Type: application/json

{
  "order_id": "ORD20250711001",
  "refund_amount": 50.00,
  "reason": "部分退款"
}
```

成功响应：

```json
{
  "refund_id": "mock_rf_ORD20250711001",
  "status": "SUCCESS"
}
```

## 错误处理

| 状态码 | 说明 | 常见原因 |
|--------|------|----------|
| 400 | 请求参数错误 | JSON 格式错误、缺少必填字段、order_id 为空 |
| 405 | HTTP 方法不允许 | 对 GET 端点使用 POST 或反之 |
| 500 | 服务端错误 | 支付提供商服务不可用、退款被拒绝、订单不存在 |

所有错误响应均为 JSON 格式：

```json
{
  "error": "invalid request body: unexpected end of JSON input"
}
```

## 快速集成示例

### Go

```go
type PayClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *PayClient) CreateOrder(orderID string, amount float64, subject string) (*PayResponse, error) {
    body, _ := json.Marshal(PayRequest{
        OrderID: orderID, Amount: amount, Subject: subject,
    })
    resp, err := c.httpClient.Post(c.baseURL+"/pay", "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var result PayResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

func (c *PayClient) QueryOrder(orderID string) (*OrderStatus, error) {
    resp, err := c.httpClient.Get(c.baseURL + "/query/" + orderID)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var status OrderStatus
    json.NewDecoder(resp.Body).Decode(&status)
    return &status, nil
}

func (c *PayClient) Refund(orderID string, amount float64, reason string) (*RefundResponse, error) {
    body, _ := json.Marshal(RefundRequest{
        OrderID: orderID, RefundAmount: amount, Reason: reason,
    })
    resp, err := c.httpClient.Post(c.baseURL+"/refund", "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var result RefundResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}
```

### Python

```python
import httpx


class PayClient:
    def __init__(self, base_url: str):
        self.base_url = base_url
        self.http = httpx.Client()

    def create_order(self, order_id: str, amount: float, subject: str) -> dict:
        resp = self.http.post(f"{self.base_url}/pay", json={
            "order_id": order_id,
            "amount": amount,
            "subject": subject,
        })
        resp.raise_for_status()
        return resp.json()

    def query_order(self, order_id: str) -> dict:
        resp = self.http.get(f"{self.base_url}/query/{order_id}")
        resp.raise_for_status()
        return resp.json()

    def refund(self, order_id: str, refund_amount: float, reason: str = "") -> dict:
        resp = self.http.post(f"{self.base_url}/refund", json={
            "order_id": order_id,
            "refund_amount": refund_amount,
            "reason": reason,
        })
        resp.raise_for_status()
        return resp.json()
```

### 开发调试

开发环境使用 `apiMockProvider`，无需真实支付凭证即可测试完整支付流程。
