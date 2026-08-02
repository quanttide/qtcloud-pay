/// 对账与银行流水比对（纯逻辑，可独立测试）。
///
/// 余额一致性：余额 = 交易按方向求和（充值 +、消费/核销 −、发券 0）。
/// 对公打款核对：充值登记 vs 银行流水 CSV（按日期 + 金额匹配）。
use csv::ReaderBuilder;
use serde::Serialize;

use crate::error::CliError;
use crate::models::transaction::Transaction;
use crate::money;

/// 返回 余额 − 交易求和（0 = 一致；正数 = 余额多，负数 = 余额少）。
pub fn balance_diff(balance_cents: i64, transactions: &[Transaction]) -> i64 {
    let sum: i64 = transactions.iter().map(|t| t.amount_cents).sum();
    balance_cents - sum
}

/// 银行流水行（CSV 解析后）
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BankRow {
    pub date: String,
    pub amount_cents: i64,
    pub remark: String,
}

/// 充值登记行（交易流水中 kind = recharge 的投影）
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct RechargeRow {
    pub date: String,
    pub amount_cents: i64,
    pub source: String,
}

/// 银行流水列名映射，默认 date/amount/remark。
/// 银行 CSV 列名不同时用 --bank-cols 映射，如 "date=交易日期,amount=金额,remark=备注"。
#[derive(Debug, Clone)]
pub struct BankCols {
    pub date: String,
    pub amount: String,
    pub remark: String,
}

impl Default for BankCols {
    fn default() -> Self {
        BankCols {
            date: "date".into(),
            amount: "amount".into(),
            remark: "remark".into(),
        }
    }
}

impl BankCols {
    pub fn parse(spec: &str) -> Result<BankCols, String> {
        let mut cols = BankCols::default();
        for part in spec.split(',') {
            let part = part.trim();
            if part.is_empty() {
                continue;
            }
            let (key, value) = part
                .split_once('=')
                .ok_or_else(|| format!("列名映射格式非法：{part}（应为 字段=列名）"))?;
            match key.trim() {
                "date" => cols.date = value.trim().to_string(),
                "amount" => cols.amount = value.trim().to_string(),
                "remark" => cols.remark = value.trim().to_string(),
                other => return Err(format!("未知字段：{other}（支持 date / amount / remark）")),
            }
        }
        Ok(cols)
    }
}

/// 对公打款核对结果
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize)]
pub struct BankDiff {
    pub recharge_count: usize,
    pub bank_count: usize,
    pub recharge_sum: i64,
    pub bank_sum: i64,
    pub unmatched_recharges: Vec<RechargeRow>,
    pub unmatched_bank: Vec<BankRow>,
}

impl BankDiff {
    pub fn matched(&self) -> bool {
        self.unmatched_recharges.is_empty() && self.unmatched_bank.is_empty()
    }
}

/// 充值登记 vs 银行流水（按日期 + 金额贪心匹配）。
pub fn diff_bank(recharges: &[RechargeRow], bank: &[BankRow]) -> BankDiff {
    let mut bank_left: Vec<BankRow> = bank.to_vec();
    let mut matched = vec![false; recharges.len()];
    for (i, r) in recharges.iter().enumerate() {
        if let Some(pos) = bank_left
            .iter()
            .position(|b| b.date == r.date && b.amount_cents == r.amount_cents)
        {
            matched[i] = true;
            bank_left.remove(pos);
        }
    }
    BankDiff {
        recharge_count: recharges.len(),
        bank_count: bank.len(),
        recharge_sum: recharges.iter().map(|r| r.amount_cents).sum(),
        bank_sum: bank.iter().map(|b| b.amount_cents).sum(),
        unmatched_recharges: recharges
            .iter()
            .enumerate()
            .filter(|(i, _)| !matched[*i])
            .map(|(_, r)| r.clone())
            .collect(),
        unmatched_bank: bank_left,
    }
}

/// 解析银行流水 CSV（首行为表头，金额为元，支持小数）。
pub fn parse_bank_csv(path: &str, cols: &BankCols) -> Result<Vec<BankRow>, CliError> {
    let content = std::fs::read_to_string(path)
        .map_err(|e| CliError::Fatal(format!("读取银行流水 {path} 失败：{e}")))?;
    parse_bank_csv_str(&content, cols, path)
}

pub fn parse_bank_csv_str(
    content: &str,
    cols: &BankCols,
    source: &str,
) -> Result<Vec<BankRow>, CliError> {
    let mut reader = ReaderBuilder::new()
        .flexible(true)
        .from_reader(content.as_bytes());
    let headers = reader
        .headers()
        .map_err(|e| CliError::Fatal(format!("解析银行流水表头失败：{e}")))?;
    let date_i = headers.iter().position(|h| h == cols.date).ok_or_else(|| {
        CliError::Usage(format!(
            "银行 CSV 缺少列 {}（可用 --bank-cols 映射）",
            cols.date
        ))
    })?;
    let amount_i = headers
        .iter()
        .position(|h| h == cols.amount)
        .ok_or_else(|| {
            CliError::Usage(format!(
                "银行 CSV 缺少列 {}（可用 --bank-cols 映射）",
                cols.amount
            ))
        })?;
    let remark_i = headers.iter().position(|h| h == cols.remark);

    let mut rows = Vec::new();
    for (line, record) in reader.records().enumerate() {
        let record = record
            .map_err(|e| CliError::Fatal(format!("解析银行流水第 {} 行失败：{e}", line + 2)))?;
        let date = record.get(date_i).unwrap_or("").trim().to_string();
        let amount_text = record.get(amount_i).unwrap_or("").trim();
        let amount_cents = money::parse_yuan_to_cents(amount_text)
            .map_err(|e| CliError::Usage(format!("{} 第 {} 行：{e}", source, line + 2)))?;
        let remark = remark_i
            .and_then(|i| record.get(i))
            .unwrap_or("")
            .trim()
            .to_string();
        rows.push(BankRow {
            date,
            amount_cents,
            remark,
        });
    }
    Ok(rows)
}

/// 从交易流水中提取充值登记行（kind = recharge）。
pub fn recharge_rows(transactions: &[Transaction]) -> Vec<RechargeRow> {
    transactions
        .iter()
        .filter(|t| t.kind == "recharge")
        .map(|t| RechargeRow {
            date: t.occurred_at.chars().take(10).collect(),
            amount_cents: t.amount_cents,
            source: t.source.clone(),
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use crate::models::transaction::Transaction;

    use super::*;

    fn txn(kind: &str, amount: i64, time: &str) -> Transaction {
        Transaction {
            id: "tx".into(),
            account_id: "acc".into(),
            kind: kind.into(),
            amount_cents: amount,
            occurred_at: time.into(),
            source: "src".into(),
        }
    }

    #[test]
    fn balance_diff_works() {
        let txns = vec![
            txn("recharge", 10000, "2026-08-02 10:00:00"),
            txn("consume", -2000, "2026-08-02 10:10:00"),
        ];
        assert_eq!(balance_diff(8000, &txns), 0);
        assert_eq!(balance_diff(8200, &txns), 200);
        assert_eq!(balance_diff(7800, &txns), -200);
    }

    #[test]
    fn bank_diff_matches_by_date_and_amount() {
        let recharges = vec![RechargeRow {
            date: "2026-08-02".into(),
            amount_cents: 10000,
            source: "R-1".into(),
        }];
        let bank = vec![
            BankRow {
                date: "2026-08-02".into(),
                amount_cents: 10000,
                remark: "已登记".into(),
            },
            BankRow {
                date: "2026-08-02".into(),
                amount_cents: 5000,
                remark: "未登记".into(),
            },
        ];
        let diff = diff_bank(&recharges, &bank);
        assert!(!diff.matched());
        assert!(diff.unmatched_recharges.is_empty());
        assert_eq!(diff.unmatched_bank.len(), 1);
        assert_eq!(diff.unmatched_bank[0].amount_cents, 5000);
    }

    #[test]
    fn parse_bank_csv_with_cols_mapping() {
        let csv = "交易日期,金额,备注\n2026-08-02,100.00,对公打款\n2026-08-03,50.5,\n";
        let cols = BankCols::parse("date=交易日期,amount=金额,remark=备注").unwrap();
        let rows = parse_bank_csv_str(csv, &cols, "bank.csv").unwrap();
        assert_eq!(rows.len(), 2);
        assert_eq!(rows[0].amount_cents, 10000);
        assert_eq!(rows[1].amount_cents, 5050);
    }
}
