# 用户指南 · 接入 qtcloud-pay

## 概述

qtcloud-pay v0.1.0 提供**账本核心**：账户（虚拟钱包）、交易账本、优惠券/代金券、订单结算、对账。设计上**先记账、后接支付**——对公打款登记入账即可跑通业务闭环，支付渠道（微信/支付宝）在 v0.2.0 接入后作为自动入账来源，账本模型不变。

三个业务（量潮课堂 / 量潮数据 / 量潮云）共享同一套 API 与计费引擎，差别只是方案参数。

**金额约定**：所有金额均为**整数分**（100 元 = `10000`）。

## 前置条件

- 获取服务地址（如 `https://pay.quanttide.com`）
- 商户开通账户，由运营在系统内创建客户账户

## 业务模型（方案契约 v0.1）

三业务计费同构：本质都是「单价 × 数量」，按各自节奏扣费：

| 业务 | 付费方式 | 扣费方式 | 激励 |
|------|---------|---------|------|
| 量潮课堂 | 对公打款，付费记入额度 | 按学习进度逐次扣费（学一节扣一节） | 按课程交付发放折扣券/代金券 |
| 量潮数据 | 预收（按预估合同额） | 按交付进度分期扣费、按实际数据量弹性计费 | 满减券/代金券 |
| 量潮云 | 预存 | 按量多次小额消费 | 全场代金券 |

结算（扣费）时，多退少补：按实际用量结算后，多收的通过**退款登记**原路退回，少收的通过**补款充值**补齐。

### 计费规则（系统级，v0.1 生效）

结算时按以下顺序与力度规则抵扣，**由系统统一执行，接入方与客户无需配置**：

1. **抵扣顺序**：满减券 → 折扣券 → 代金券 → 余额
2. **多券选力度最大**：同类型多张可用券时，满减选减额最大、折扣选省得最多
3. **代金券先于余额**：券抵扣完后，代金券优先于余额抵现
4. **代金券不找零**：代金券面值大于剩余应付时只抵应付，差额不退

> 券的折扣比例、满减额度、面值等是**发放时参数**，由运营活动决定，不属于系统规则。

## 核心旅程

### 1. 创建账户

```http
POST /accounts
Content-Type: application/json

{"customer_id": "stu-1001"}
```

成功响应（201）：

```json
{"id": "acc_3f2a...", "customer_id": "stu-1001", "balance": 0}
```

### 2. 充值登记（付费记额度）

对公打款到账后登记入账；`voucher_no`（打款凭证号）为幂等键，重复提交不会重复入账。

```http
POST /accounts/{id}/recharges
Content-Type: application/json

{"amount": 20000, "voucher_no": "GT-001"}
```

### 3. 发放优惠券 / 代金券

批量发放，`batch_no` 为幂等键，同批次重复提交不会重复发放。

```http
POST /accounts/{id}/coupons
Content-Type: application/json

{
  "type": "discount",          // discount=折扣券（rate 为力度，9 折 = 90）
  "rate": 90,
  "scope": "course",           // all / course / data / cloud / product
  "expires_at": "2026-08-04T00:00:00Z",
  "count": 10,
  "batch_no": "GT-B-001"
}
```

满减券：`{"type": "full_reduction", "threshold": 10000, "amount": 2000, ...}`（应付 ≥ 门槛时减 `amount`）。

```http
POST /accounts/{id}/vouchers
Content-Type: application/json

{"amount": 2000, "scope": "all", "expires_at": "2026-08-04T00:00:00Z", "batch_no": "GT-V-001"}
```

### 4. 结算扣费

下单并结算：按计费规则自动抵扣（券 → 代金券 → 余额）；`order_id` 为幂等键，重复提交返回同一订单。

```http
POST /orders
Content-Type: application/json

{
  "order_id": "O-GT-1",
  "account_id": "acc_3f2a...",
  "scope": "course",
  "amount": 10000
}
```

成功响应（201）含结算明细（即抵扣行）：

```json
{
  "id": "O-GT-1",
  "status": "settled",
  "settle_detail": [
    {"kind": "coupon", "ref_id": 1, "amount": 1000},
    {"kind": "voucher", "ref_id": 1, "amount": 2000},
    {"kind": "balance", "ref_id": 0, "amount": 7000}
  ]
}
```

余额不足时返回 **422**，整体回滚（无订单、无扣减、无交易写入）。

### 5. 退款登记（多退）

按实际用量结算后多收的款项，原路退回；`voucher_no` 为幂等键。

```http
POST /accounts/{id}/refunds
Content-Type: application/json

{"amount": 100000, "voucher_no": "SJ-R-001"}
```

余额不足时返回 422，整体回滚。

### 6. 查询与对账

| 端点 | 说明 |
|------|------|
| `GET /accounts/{id}` | 账户与余额 |
| `GET /accounts/{id}/transactions?limit=20&offset=0` | 交易流水（倒序） |
| `GET /accounts/{id}/coupons`、`GET /accounts/{id}/vouchers` | 我的券（含状态） |
| `GET /orders/{id}` | 订单与结算明细 |
| `GET /accounts/{id}/statement` | 账单导出（期初/运行余额/期末） |
| `GET /reconcile/consistency` | 余额-交易一致性校验（余额 = Σ充值 − Σ退款 − Σ余额支付） |
| `POST /reconcile/bank` | 对公打款核对（银行流水 CSV：`voucher_no,amount_cents,date`） |

券状态：`issued`（已发放）/ `used`（已使用）/ `expired`（已过期，查询时惰性流转）。

## 错误处理

| 状态码 | 场景 |
|--------|------|
| 400 | 请求体非法、金额非正、缺少幂等键 |
| 404 | 账户/订单不存在 |
| 409 | 账户已存在、券不可用 |
| 422 | 余额不足（结算或退款），整体回滚 |
| 500 | 服务端错误 |

错误响应：

```json
{"error": "invalid request body"}
```

## 可靠性保证

- **不丢**：每笔业务动作（充值/发券/扣费/核销/退款）都落在交易账本上
- **不重**：充值/退款/发券/结算均有业务幂等键 + 唯一约束
- **不错**：余额、券状态、交易同事务更新；失败整体回滚
- **可查**：任意交易可追溯（订单号/券关联），账单可导出

## 支付渠道（v0.2.0）

当前版本**不接入支付**——模拟账户模式下，打款登记即完成闭环。v0.2.0 将接入微信 JSAPI 与支付宝网页支付：支付回调验签后自动写入充值交易（幂等键 = 渠道交易号），替代手动登记，**账本模型不变**。
