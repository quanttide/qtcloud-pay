# reconciliation 对账与可查（M4）

客户端位置：`lib/screens/reconcile_screen.dart`、`lib/widgets/reconcile_diff_table.dart`

服务端模块：[internal/reconciliation](../../provider/internal/reconciliation)（对照 [docs/reconciliation.md](../../provider/docs/reconciliation.md)）

## 职责

账本可靠性的最后一环：一致性校验（不错）、对公打款核对、账单导出（可查）。客户端是展示台：展示差异、上传银行流水 CSV、导出账单。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `reconcile_screen.dart` | 对账页：一致性校验结果（差异清单）、银行流水 CSV 导入与比对报告、账单导出 |
| `ReconcileDiffTable` | 差异表：差异行定位（账户、余额 vs 期望值）+ 跳转该账户流水 |
| `TransactionList` | 账单明细复用流水组件（含 `running_balance` 列） |

## 模型（JSON 契约与服务端一致）

```dart
/// 一致性校验差异项（GET /reconcile/consistency）。
class Discrepancy {
  final String accountId; // json: account_id
  final int balance;      // 账户当前余额（分）
  final int expected;     // 由交易推导的余额（分）

  Discrepancy.fromJson(Map<String, dynamic> json)
      : accountId = json['account_id'],
        balance = json['balance'],
        expected = json['expected'];
}

/// 对公打款核对报告（POST /reconcile/bank）。
class BankReport {
  final int total;
  final List<BankMatch> matched;
  final List<BankUnmatch> unmatched;
  // BankMatch: {row: BankRow, transaction_id}
  // BankUnmatch: {row: BankRow, reason}
}

/// 银行流水 CSV 行。
class BankRow {
  final String voucherNo; // json: voucher_no 凭证号
  final int amount;       // 金额（分）
  final String date;      // 日期 YYYY-MM-DD
}

/// 账单（GET /accounts/{id}/statement）。
class Statement {
  final String accountId;      // json: account_id
  final int openingBalance;    // json: opening_balance 期初余额（分）
  final int closingBalance;    // json: closing_balance 期末余额（分）
  final List<StatementEntry> entries;
  final DateTime generatedAt;  // json: generated_at
}

class StatementEntry {
  final int id;
  final String type;            // 同交易类型枚举
  final int amount;             // 金额（分）
  final String? note;
  final DateTime createdAt;     // json: created_at
  final int runningBalance;     // json: running_balance 流水后余额
}
```

## API

| 方法 | 路径 | 请求 | 响应 |
|------|------|------|------|
| GET | `/reconcile/consistency` | — | 200 `{"discrepancies": [...]}` |
| POST | `/reconcile/bank` | body = CSV 文件 | 200 `BankReport` |
| GET | `/accounts/{id}/statement` | — | 200 `Statement` |

## 关键点（联调）

- 一致性校验：`balance`（余额字段）vs `expected`（Σ充值 − Σ余额支付）——差异行即「余额对不上」的定位入口，引导到工作台 §五
- 银行流水 CSV 格式：每行 `凭证号,金额(分),日期(YYYY-MM-DD)`（与服务端 `BankRow` 解析一致），客户端上传前本地校验格式
- 账单导出：期初 + 流水 + 期末，`entries[].running_balance` 与 `opening_balance`、`closing_balance` 自洽——联调验收点
- 余额求和约定沿用 [transaction.md](transaction.md)：`issue`/`redeem` 不参与

## 测试

widget 测试：差异表渲染与跳转、CSV 解析校验、Statement 期初期末自洽展示。
