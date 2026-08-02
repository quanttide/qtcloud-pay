// 对账与可查模型。JSON 契约与服务端 reconciliation 模块一致。

/// 一致性校验差异项（GET /reconcile/consistency）。
class Discrepancy {
  final String accountId; // json: account_id
  final int balance; // 账户当前余额（分）
  final int expected; // 由交易推导的余额（分）

  const Discrepancy({
    required this.accountId,
    required this.balance,
    required this.expected,
  });

  factory Discrepancy.fromJson(Map<String, dynamic> json) => Discrepancy(
        accountId: json['account_id'] as String,
        balance: json['balance'] as int,
        expected: json['expected'] as int,
      );
}

/// 银行流水 CSV 行。
class BankRow {
  final String voucherNo; // json: voucher_no 凭证号
  final int amount; // 金额（分）
  final String date; // 日期 YYYY-MM-DD

  const BankRow({
    required this.voucherNo,
    required this.amount,
    required this.date,
  });

  factory BankRow.fromJson(Map<String, dynamic> json) => BankRow(
        voucherNo: json['voucher_no'] as String,
        amount: json['amount'] as int,
        date: json['date'] as String,
      );
}

/// 与充值交易匹配成功的流水行。
class BankMatch {
  final BankRow row;
  final int transactionId; // json: transaction_id

  const BankMatch({required this.row, required this.transactionId});

  factory BankMatch.fromJson(Map<String, dynamic> json) => BankMatch(
        row: BankRow.fromJson(json['row'] as Map<String, dynamic>),
        transactionId: json['transaction_id'] as int,
      );
}

/// 未能匹配的流水行及原因。
class BankUnmatch {
  final BankRow row;
  final String reason;

  const BankUnmatch({required this.row, required this.reason});

  factory BankUnmatch.fromJson(Map<String, dynamic> json) => BankUnmatch(
        row: BankRow.fromJson(json['row'] as Map<String, dynamic>),
        reason: json['reason'] as String,
      );
}

/// 对公打款核对报告（POST /reconcile/bank）。
class BankReport {
  final int total;
  final List<BankMatch> matched;
  final List<BankUnmatch> unmatched;

  const BankReport({
    required this.total,
    required this.matched,
    required this.unmatched,
  });

  factory BankReport.fromJson(Map<String, dynamic> json) => BankReport(
        total: json['total'] as int,
        matched: (json['matched'] as List? ?? [])
            .map((e) => BankMatch.fromJson(e as Map<String, dynamic>))
            .toList(),
        unmatched: (json['unmatched'] as List? ?? [])
            .map((e) => BankUnmatch.fromJson(e as Map<String, dynamic>))
            .toList(),
      );
}

/// 账单明细行。
class StatementEntry {
  final int id;
  final String type; // 同交易类型枚举
  final int amount; // 金额（分）
  final String? note;
  final DateTime createdAt; // json: created_at
  final int runningBalance; // json: running_balance 流水后余额

  const StatementEntry({
    required this.id,
    required this.type,
    required this.amount,
    this.note,
    required this.createdAt,
    required this.runningBalance,
  });

  factory StatementEntry.fromJson(Map<String, dynamic> json) => StatementEntry(
        id: json['id'] as int,
        type: json['type'] as String,
        amount: json['amount'] as int,
        note: json['note'] as String?,
        createdAt: DateTime.parse(json['created_at'] as String),
        runningBalance: json['running_balance'] as int,
      );
}

/// 账单（GET /accounts/{id}/statement）。
class Statement {
  final String accountId; // json: account_id
  final int openingBalance; // json: opening_balance 期初余额（分）
  final int closingBalance; // json: closing_balance 期末余额（分）
  final List<StatementEntry> entries;
  final DateTime generatedAt; // json: generated_at

  const Statement({
    required this.accountId,
    required this.openingBalance,
    required this.closingBalance,
    required this.entries,
    required this.generatedAt,
  });

  factory Statement.fromJson(Map<String, dynamic> json) => Statement(
        accountId: json['account_id'] as String,
        openingBalance: json['opening_balance'] as int,
        closingBalance: json['closing_balance'] as int,
        entries: (json['entries'] as List? ?? [])
            .map((e) => StatementEntry.fromJson(e as Map<String, dynamic>))
            .toList(),
        generatedAt: DateTime.parse(json['generated_at'] as String),
      );
}
