# 核心旅程

从开户到扣费退款的完整写操作链路，按顺序执行：

```
创建账户 → 充值登记 → 发放券 → 结算扣费 → 退款登记
```

## 1. 创建账户

```http
POST /accounts
Content-Type: application/json

{"customer_id": "stu-1001"}
```

成功响应（201）：

```json
{"id": "acc_3f2a...", "customer_id": "stu-1001", "balance": 0}
```

## 2. 充值登记（付费记额度）

对公打款到账后登记入账；`voucher_no`（打款凭证号）为幂等键，重复提交不会重复入账。

```http
POST /accounts/{id}/recharges
Content-Type: application/json

{"amount": 200.00, "voucher_no": "GT-001"}
```

## 3. 发放优惠券 / 代金券

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

满减券：`{"type": "full_reduction", "threshold": 100.00, "amount": 20.00, ...}`（应付 ≥ 门槛时减 `amount`）。

```http
POST /accounts/{id}/vouchers
Content-Type: application/json

{"amount": 20.00, "scope": "all", "expires_at": "2026-08-04T00:00:00Z", "batch_no": "GT-V-001"}
```

## 4. 结算扣费

下单并结算：按[计费规则](billing.md)自动抵扣（券 → 代金券 → 余额）；`order_id` 为幂等键，重复提交返回同一订单。

```http
POST /orders
Content-Type: application/json

{
  "order_id": "O-GT-1",
  "account_id": "acc_3f2a...",
  "scope": "course",
  "amount": 100.00
}
```

成功响应（201）含结算明细（即抵扣行）：

```json
{
  "id": "O-GT-1",
  "status": "settled",
  "settle_detail": [
    {"kind": "coupon", "ref_id": 1, "amount": 10.00},
    {"kind": "voucher", "ref_id": 1, "amount": 20.00},
    {"kind": "balance", "ref_id": 0, "amount": 70.00}
  ]
}
```

余额不足时返回 **422**，整体回滚（无订单、无扣减、无交易写入）。

## 5. 退款登记（多退）

按实际用量结算后多收的款项，原路退回；`voucher_no` 为幂等键。

```http
POST /accounts/{id}/refunds
Content-Type: application/json

{"amount": 1000.00, "voucher_no": "SJ-R-001"}
```

余额不足时返回 422，整体回滚。

> 少补：实际用量超过预存时，走「充值登记」（第 2 步）补款后再结算。
