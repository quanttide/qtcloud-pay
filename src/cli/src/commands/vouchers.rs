use chrono::NaiveDate;
use serde::Serialize;
use tabled::Tabled;

use crate::args::{VouchersAction, VouchersArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::voucher::IssueVoucherRequest;
use crate::money;
use crate::output;
use crate::status;

pub async fn run(ctx: &Ctx, args: VouchersArgs) -> Result<(), CliError> {
    match args.action {
        VouchersAction::Issue {
            account_id,
            amount,
            scope,
            expires_at,
            batch_no,
        } => {
            issue(
                ctx,
                &account_id,
                &amount,
                scope.as_deref(),
                expires_at.as_deref(),
                &batch_no,
            )
            .await
        }
        VouchersAction::List { account_id } => list(ctx, &account_id).await,
    }
}

/// POST /accounts/{id}/vouchers 发放代金券（幂等键 = 发放批次号）
async fn issue(
    ctx: &Ctx,
    account_id: &str,
    amount: &str,
    scope: Option<&str>,
    expires_at: Option<&str>,
    batch_no: &str,
) -> Result<(), CliError> {
    if batch_no.trim().is_empty() {
        return Err(CliError::Usage(
            "发放批次号（--batch-no）必填，作为幂等键".to_string(),
        ));
    }
    let amount_cents = money::parse_yuan_to_cents(amount).map_err(CliError::Usage)?;
    if amount_cents <= 0 {
        return Err(CliError::Usage("代金券面值必须大于 0".to_string()));
    }
    if let Some(date) = expires_at {
        NaiveDate::parse_from_str(date, "%Y-%m-%d")
            .map_err(|_| CliError::Usage(format!("日期格式非法：{date}（应为 YYYY-MM-DD）")))?;
    }
    let result = ctx
        .client
        .issue_voucher(
            account_id,
            &IssueVoucherRequest {
                amount_cents,
                scope: scope.map(str::to_string),
                expires_at: expires_at.map(str::to_string),
                batch_no: batch_no.trim().to_string(),
            },
        )
        .await?;
    if ctx.json {
        return output::print_json(&result);
    }
    println!("✔ 代金券已发放（批次 {}）", batch_no.trim());
    println!("券 id\t{}", result.voucher.id);
    println!(
        "面值\t{}",
        money::format_cents_colored(result.voucher.amount_cents, true, ctx.color)
    );
    println!(
        "状态\t{}",
        status::status_chip(&result.voucher.status, ctx.color)
    );
    println!("发券交易\t{}（账本已记录，不丢）", result.transaction.id);
    Ok(())
}

#[derive(Debug, Clone, Serialize, Tabled)]
struct VoucherRow {
    #[tabled(rename = "券 id")]
    id: String,
    #[tabled(rename = "面值")]
    amount: String,
    #[tabled(rename = "适用范围")]
    scope: String,
    #[tabled(rename = "有效期")]
    expires_at: String,
    #[tabled(rename = "状态")]
    status: String,
}

/// GET /accounts/{id}/vouchers 查询代金券
async fn list(ctx: &Ctx, account_id: &str) -> Result<(), CliError> {
    let vouchers = ctx.client.list_vouchers(account_id).await?;
    if ctx.json {
        return output::print_json(&vouchers);
    }
    let rows: Vec<VoucherRow> = vouchers
        .iter()
        .map(|v| VoucherRow {
            id: v.id.clone(),
            amount: money::format_cents(v.amount_cents, false),
            scope: v.scope.clone(),
            expires_at: v.expires_at.clone(),
            status: status::status_chip(&v.status, ctx.color),
        })
        .collect();
    println!("账户 {account_id} 代金券（{} 张）", rows.len());
    println!("{}", output::table(rows));
    Ok(())
}
