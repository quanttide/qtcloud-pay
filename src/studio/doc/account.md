# account 账户与余额（M1）

客户端位置：`lib/screens/accounts_screen.dart`、`lib/screens/account_detail_screen.dart`、`lib/screens/recharge_screen.dart`

服务端模块：[internal/account](../../provider/internal/account)（对照 [docs/account.md](../../provider/docs/account.md)）

## 职责

账户（客户虚拟钱包）的创建与查询、对公打款充值登记（带幂等键）。客户端是纯操作载体：不计算余额，余额一律展示服务端返回的 `balance`。

## 页面

| 页面 | 文件 | 功能 |
|------|------|------|
| 账户页 | `accounts_screen.dart` | 账户列表、创建账户（输入 `customer_id`） |
| 账户详情页 | `account_detail_screen.dart` | 余额（MoneyText）、交易流水（TransactionList）、券列表（StatusChip） |
| 充值登记页 | `recharge_screen.dart` | 表单：AccountPicker + AmountField + IdempotencyField（打款凭证号）+ 备注 |

## 组件

`AccountPicker`（账户选择）、`MoneyText`（余额展示）、`TransactionList`（流水）、`AmountField`（金额输入）、`IdempotencyField`（凭证号）、`StatusChip`（券状态）。

## 模型（JSON 契约与服务端一致）

```dart
class Account {
  final String id;          // 业务号，如 acc_xxx
  final String customerId;  // json: customer_id
  final int balance;        // 余额（分），交易的投影
  final DateTime createdAt; // json: created_at
  final DateTime updatedAt; // json: updated_at

  Account.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        customerId = json['customer_id'],
        balance = json['balance'],
        createdAt = DateTime.parse(json['created_at']),
        updatedAt = DateTime.parse(json['updated_at']);
}
```

## API

| 方法 | 路径 | 请求 | 响应 |
|------|------|------|------|
| POST | `/accounts` | `{"customer_id": "cus_xxx"}` | 201 `Account` |
| POST | `/accounts/{id}/recharges` | `{"amount": 10000, "voucher_no": "凭证号", "note": "备注"}` | 200 `{"account_id": "..."}` |
| GET | `/accounts/{id}` | — | 200 `Account` |
| GET | `/accounts/{id}/transactions` | `?limit=20&offset=0` | 200 `{"account_id", "transactions": [...]}` |

## 关键点（联调）

- `amount` 为整数分；`voucher_no` 必填（服务端缺省返回 400 `account: voucher no required`）
- 重复提交同 `voucher_no`：服务端幂等返回成功，客户端提示「该凭证号已入账，未重复登记」，不当作错误
- 账户详情页余额用 `MoneyText`（分→元，无浮点）；交易流水分页拉取（limit 默认 20、上限 100）
- 创建账户重复 `customer_id` → 409 已存在（服务端 `account: already exists`）

## 测试

widget 测试：充值表单凭证号必填校验、金额转分、账户列表与详情渲染（mock `pay_api`）。
