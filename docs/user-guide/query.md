# 查询与对账

## 端点一览

| 端点 | 说明 |
|------|------|
| `GET /accounts/{id}` | 账户与余额 |
| `GET /accounts/{id}/transactions?limit=20&offset=0` | 交易流水（倒序） |
| `GET /accounts/{id}/coupons`、`GET /accounts/{id}/vouchers` | 我的券（含状态） |
| `GET /orders/{id}` | 订单与结算明细 |
| `GET /accounts/{id}/statement` | 账单导出（期初/运行余额/期末） |
| `GET /reconcile/consistency` | 余额-交易一致性校验（余额 = Σ充值 − Σ退款 − Σ余额支付） |
| `POST /reconcile/bank` | 对公打款核对（银行流水 CSV：`voucher_no,amount_cents,date`） |

## 券状态

`issued`（已发放）/ `used`（已使用）/ `expired`（已过期，查询时惰性流转）。

## 交易类型

| 类型 | 含义 | 影响余额 |
|------|------|---------|
| `recharge` | 充值（对公打款入账） | + |
| `refund` | 退款（多退登记） | − |
| `consume` | 消费（余额支付部分） | − |
| `issue` | 发券（信息性记录） | 无 |
| `redeem` | 核销/抵现（券抵扣部分） | 无 |
