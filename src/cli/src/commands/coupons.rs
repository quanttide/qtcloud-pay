use chrono::NaiveDate;
use serde::Serialize;
use tabled::Tabled;

use crate::args::{CouponsAction, CouponsArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::coupon::{Coupon, IssueCouponRequest};
use crate::money;
use crate::output;
use crate::status;

pub async fn run(ctx: &Ctx, args: CouponsArgs) -> Result<(), CliError> {
    match args.action {
        CouponsAction::Issue {
            account_id,
            kind,
            rate,
            threshold,
            amount,
            scope,
            expires_at,
            batch_no,
        } => {
            issue(
                ctx,
                &account_id,
                &kind,
                rate,
                threshold.as_deref(),
                amount.as_deref(),
                scope.as_deref(),
                expires_at.as_deref(),
                &batch_no,
            )
            .await
        }
        CouponsAction::List { account_id } => list(ctx, &account_id).await,
    }
}

/// POST /accounts/{id}/coupons 发放优惠券（幂等键 = 发放批次号）
#[allow(clippy::too_many_arguments)]
async fn issue(
    ctx: &Ctx,
    account_id: &str,
    kind: &str,
    rate: Option<f64>,
    threshold: Option<&str>,
    amount: Option<&str>,
    scope: Option<&str>,
    expires_at: Option<&str>,
    batch_no: &str,
) -> Result<(), CliError> {
    if batch_no.trim().is_empty() {
        return Err(CliError::Usage(
            "发放批次号（--batch-no）必填，作为幂等键".to_string(),
        ));
    }
    let kind = kind.to_lowercase();
    let (rate, threshold_cents, amount_cents) = match kind.as_str() {
        "rate" => {
            let rate =
                rate.ok_or_else(|| CliError::Usage("折扣券需要 --rate（如 0.9）".to_string()))?;
            if !(0.0 < rate && rate < 1.0) {
                return Err(CliError::Usage(
                    "折扣率应在 0 到 1 之间（如 9 折 = 0.9）".to_string(),
                ));
            }
            (Some(rate), None, None)
        }
        "threshold" => {
            let threshold = money::parse_yuan_to_cents(threshold.ok_or_else(|| {
                CliError::Usage("满减券需要 --threshold（门槛，元）".to_string())
            })?)
            .map_err(CliError::Usage)?;
            let amount =
                money::parse_yuan_to_cents(amount.ok_or_else(|| {
                    CliError::Usage("满减券需要 --amount（减额，元）".to_string())
                })?)
                .map_err(CliError::Usage)?;
            if amount <= 0 || amount >= threshold {
                return Err(CliError::Usage("满减券要求 0 < 减额 < 门槛".to_string()));
            }
            (None, Some(threshold), Some(amount))
        }
        other => {
            return Err(CliError::Usage(format!(
                "券类型非法：{other}（支持 rate / threshold）"
            )))
        }
    };
    if let Some(date) = expires_at {
        validate_date(date)?;
    }
    let result = ctx
        .client
        .issue_coupon(
            account_id,
            &IssueCouponRequest {
                kind,
                rate,
                threshold_cents,
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
    println!("✔ 优惠券已发放（批次 {}）", batch_no.trim());
    println!("券 id\t{}", result.coupon.id);
    println!(
        "状态\t{}",
        status::status_chip(&result.coupon.status, ctx.color)
    );
    println!("发券交易\t{}（账本已记录，不丢）", result.transaction.id);
    Ok(())
}

#[derive(Debug, Clone, Serialize, Tabled)]
struct CouponRow {
    #[tabled(rename = "券 id")]
    id: String,
    #[tabled(rename = "类型")]
    kind: String,
    #[tabled(rename = "参数")]
    params: String,
    #[tabled(rename = "适用范围")]
    scope: String,
    #[tabled(rename = "有效期")]
    expires_at: String,
    #[tabled(rename = "状态")]
    status: String,
}

impl CouponRow {
    fn from_coupon(c: &Coupon, color: bool) -> Self {
        let (kind, params) = match c.kind.as_str() {
            "rate" => ("折扣券", format!("{:.1} 折", c.rate.unwrap_or(0.0) * 10.0)),
            "threshold" => (
                "满减券",
                format!(
                    "满 {} 减 {}",
                    money::format_cents(c.threshold_cents.unwrap_or(0), false),
                    money::format_cents(c.amount_cents.unwrap_or(0), false)
                ),
            ),
            other => (other, String::new()),
        };
        CouponRow {
            id: c.id.clone(),
            kind: kind.to_string(),
            params,
            scope: c.scope.clone(),
            expires_at: c.expires_at.clone(),
            status: status::status_chip(&c.status, color),
        }
    }
}

/// GET /accounts/{id}/coupons 查询优惠券
async fn list(ctx: &Ctx, account_id: &str) -> Result<(), CliError> {
    let coupons = ctx.client.list_coupons(account_id).await?;
    if ctx.json {
        return output::print_json(&coupons);
    }
    let rows: Vec<CouponRow> = coupons
        .iter()
        .map(|c| CouponRow::from_coupon(c, ctx.color))
        .collect();
    println!("账户 {account_id} 优惠券（{} 张）", rows.len());
    println!("{}", output::table(rows));
    Ok(())
}

fn validate_date(s: &str) -> Result<(), CliError> {
    NaiveDate::parse_from_str(s, "%Y-%m-%d")
        .map(|_| ())
        .map_err(|_| CliError::Usage(format!("日期格式非法：{s}（应为 YYYY-MM-DD）")))
}
