use qtcloud_pay_cli::models::transaction::Transaction;
use qtcloud_pay_cli::reconcile::{
    balance_diff, diff_bank, parse_bank_csv_str, recharge_rows, BankCols, BankRow, RechargeRow,
};

fn txn(kind: &str, amount: i64) -> Transaction {
    Transaction {
        id: "tx".into(),
        account_id: "acc".into(),
        kind: kind.into(),
        amount_cents: amount,
        occurred_at: "2026-08-02 10:00:00".into(),
        source: "src".into(),
    }
}

#[test]
fn balance_matches_transaction_sum() {
    let txns = vec![txn("recharge", 10000), txn("consume", -2000)];
    assert_eq!(balance_diff(8000, &txns), 0);
    assert_eq!(balance_diff(8200, &txns), 200);
    assert_eq!(balance_diff(7800, &txns), -200);
}

#[test]
fn recharge_rows_extracts_recharges() {
    let txns = vec![txn("recharge", 10000), txn("consume", -2000)];
    let rows = recharge_rows(&txns);
    assert_eq!(rows.len(), 1);
    assert_eq!(rows[0].amount_cents, 10000);
}

#[test]
fn bank_diff_reports_unmatched_both_sides() {
    let recharges = vec![RechargeRow { date: "2026-08-02".into(), amount_cents: 10000, source: "R-1".into() }];
    let bank = vec![
        BankRow { date: "2026-08-02".into(), amount_cents: 10000, remark: "已登记".into() },
        BankRow { date: "2026-08-02".into(), amount_cents: 5000, remark: "未登记".into() },
    ];
    let diff = diff_bank(&recharges, &bank);
    assert!(!diff.matched());
    assert!(diff.unmatched_recharges.is_empty());
    assert_eq!(diff.unmatched_bank.len(), 1);
    assert_eq!(diff.unmatched_bank[0].amount_cents, 5000);
}

#[test]
fn bank_csv_parses_with_column_mapping() {
    let csv = "交易日期,金额,备注\n2026-08-02,100.00,对公打款\n2026-08-03,50.5,\n";
    let cols = BankCols::parse("date=交易日期,amount=金额,remark=备注").unwrap();
    let rows = parse_bank_csv_str(csv, &cols, "bank.csv").unwrap();
    assert_eq!(rows.len(), 2);
    assert_eq!(rows[0].amount_cents, 10000);
    assert_eq!(rows[1].amount_cents, 5050);
}
