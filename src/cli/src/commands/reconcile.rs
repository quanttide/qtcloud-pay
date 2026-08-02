use serde::Serialize;
use tabled::Tabled;

use crate::args::ReconcileArgs;
use crate::commands::Ctx;
use crate::error::CliError;
use crate::output;
use crate::reconcile::{balance_diff, diff_bank, parse_bank_csv, recharge_rows, BankCols};

pub async fn run(ctx: &Ctx, args: ReconcileArgs) -> Result<(), CliError> {
    // 1. 余额一致性校验（余额 = 交易按方向求和）
    let mut rows = Vec::new();
    let mut recharges = Vec::new();
    let mut balance_ok = true;
    for account_id in &args.accounts {
        let account = ctx.client.get_account(account_id).await?;
        let txns = ctx.client.list_transactions(account_id).await?;
        recharges.extend(recharge_rows(&txns));
        let diff = balance_diff(account.balance_cents, &txns);
        balance_ok &= diff == 0;
        let sum: i64 = txns.iter().map(|t| t.amount_cents).sum();
        rows.push(DiffRow {
            account: account_id.clone(),
            balance: account.balance_cents.to_string(),
            sum: sum.to_string(),
            diff: if diff >= 0 {
                format!("+{diff}")
            } else {
                diff.to_string()
            },
            verdict: if diff == 0 {
                "✔".to_string()
            } else {
                "✘".to_string()
            },
        });
    }

    // 2. 对公打款核对（充值登记 vs 银行流水 CSV）
    let bank_diff = match &args.bank {
        Some(bank_path) => {
            let cols = match &args.bank_cols {
                Some(spec) => BankCols::parse(spec).map_err(CliError::Usage)?,
                None => BankCols::default(),
            };
            let bank = parse_bank_csv(bank_path, &cols)?;
            Some(diff_bank(&recharges, &bank))
        }
        None => None,
    };

    if ctx.json {
        output::print_json(&rows)?;
        if let Some(d) = &bank_diff {
            output::print_json(d)?;
        }
    } else {
        println!("余额一致性校验（余额 = 交易按方向求和）");
        println!("{}", output::table(rows));
        if let Some(diff) = &bank_diff {
            println!();
            println!("银行流水比对（对公打款核对）");
            println!(
                "充值登记 {} 笔（{} 分） vs 银行流水 {} 笔（{} 分）",
                diff.recharge_count, diff.recharge_sum, diff.bank_count, diff.bank_sum
            );
            for r in &diff.unmatched_recharges {
                println!(
                    "✘ 未匹配登记\t{} {} 分 {}",
                    r.date, r.amount_cents, r.source
                );
            }
            for b in &diff.unmatched_bank {
                println!(
                    "✘ 未匹配流水\t{} {} 分 {}",
                    b.date, b.amount_cents, b.remark
                );
            }
        }
    }

    if !balance_ok {
        return Err(CliError::Business(
            "余额 ≠ 交易求和：请导出 statement 逐笔核对，登记异常并回溯交易".to_string(),
        ));
    }
    if let Some(diff) = &bank_diff {
        if !diff.matched() {
            return Err(CliError::Business(
                "对账存在差异：请导出 statement 逐笔核对，登记异常并回溯交易".to_string(),
            ));
        }
    }
    if !ctx.quiet && !ctx.json {
        println!("✔ 对账通过");
    }
    Ok(())
}

#[derive(Debug, Clone, Serialize, Tabled)]
struct DiffRow {
    #[tabled(rename = "账户")]
    account: String,
    #[tabled(rename = "余额(分)")]
    balance: String,
    #[tabled(rename = "交易求和(分)")]
    sum: String,
    #[tabled(rename = "差异")]
    diff: String,
    #[tabled(rename = "结论")]
    verdict: String,
}
