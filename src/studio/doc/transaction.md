# transaction 交易账本（M1）

客户端位置：`lib/models/transaction.dart`、`lib/widgets/transaction_list.dart`

服务端模块：[internal/transaction](../../provider/internal/transaction)（对照 [docs/transaction.md](../../provider/docs/transaction.md)）

## 职责

交易流水的展示：类型、金额、余额快照、来源。客户端只读展示，绝不直接构造交易数据（账本写入唯一入口在服务端）。

## 组件

`TransactionList`：按时间倒序展示流水行（类型 / 金额 / 余额快照 / 时间 / 备注），任意交易可追溯（客诉可证）。

## 模型（JSON 契约与服务端一致）

```dart
class Transaction {
  final int id;
  final String accountId;   // json: account_id
  final String type;        // recharge / consume / issue / redeem
  final int amount;         // 金额（分）
  final int balanceAfter;   // json: balance_after 交易后余额快照（仅充值/消费有效）
  final String? orderId;    // json: order_id 消费/核销时关联订单
  final String? note;
  final DateTime createdAt; // json: created_at

  Transaction.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        accountId = json['account_id'],
        type = json['type'],
        amount = json['amount'],
        balanceAfter = json['balance_after'],
        orderId = json['order_id'],
        note = json['note'],
        createdAt = DateTime.parse(json['created_at']);
}

/// 带符号金额（充值 +，消费 −，发券/核销 0）——与服务端 SignedAmount 一致。
int signedAmount(String type, int amount) => switch (type) {
      'recharge' => amount,
      'consume' => -amount,
      _ => 0,
    };
```

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/accounts/{id}/transactions` | 流水查询（分页倒序，路由沿用账户资源） |

响应包装：`{"account_id": "...", "transactions": [...]}`，`transactions` 为空时返回 `[]`。

## 关键点（联调）

- 余额求和约定：余额 = Σ(充值) − Σ(余额支付部分)；发券/核销（`issue`/`redeem`）不参与余额求和——对账页与详情页的余额展示遵循此约定
- `balance_after` 为交易后快照，用于展示「交易后余额」；`issue`/`redeem` 快照无余额含义，前端可置灰
- 客户端不展示 `idempotency_key`（服务端 `json:"-"` 不返回）

## 测试

`TransactionList` 渲染四种类型与方向着色（充值 + 绿 / 消费 − 红）；`signedAmount` 方向正确性。
