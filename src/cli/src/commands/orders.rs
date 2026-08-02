use serde::Serialize;
use tabled::Tabled;

use crate::args::{OrdersAction, OrdersArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::order::{CreateOrderRequest, Order};
use crate::money;
use crate::output;
use crate::status;

pub async fn run(ctx: &Ctx, args: OrdersArgs) -> Result<(), CliError> {
    match args.action {
        OrdersAction::Create {
            account,
            amount,
            order_no,
            subject,
        } => create(ctx, &account, &amount, &order_no, subject.as_deref()).await,
        OrdersAction::Get { order_id } => get(ctx, &order_id).await,
    }
}

/// POST /orders 下单并结算（幂等键 = 订单号）
async fn create(
    ctx: &Ctx,
    account: &str,
    amount: &str,
    order_no: &str,
    subject: Option<&str>,
) -> Result<(), CliError> {
    let amount_cents = money::parse_yuan_to_cents(amount).map_err(CliError::Usage)?;
    if amount_cents <= 0 {
        return Err(CliError::Usage("订单金额必须大于 0".to_string()));
    }
    if order_no.trim().is_empty() {
        return Err(CliError::Usage(
            "订单号（--order-no）必填，作为幂等键".to_string(),
        ));
    }
    let result = ctx
        .client
        .create_order(&CreateOrderRequest {
            account_id: account.to_string(),
            amount_cents,
            order_no: order_no.trim().to_string(),
            subject: subject.map(str::to_string),
        })
        .await?;
    if ctx.json {
        return output::print_json(&result);
    }
    println!("✔ 订单已结算（订单号 {}）", order_no.trim());
    print_order(ctx, &result.order);
    println!(
        "余额\t{}",
        money::format_cents_colored(result.account.balance_cents, true, ctx.color)
    );
    Ok(())
}

/// GET /orders/{id} 订单与结算明细
async fn get(ctx: &Ctx, order_id: &str) -> Result<(), CliError> {
    let order = ctx.client.get_order(order_id).await?;
    if ctx.json {
        return output::print_json(&order);
    }
    print_order(ctx, &order);
    Ok(())
}

#[derive(Debug, Clone, Serialize, Tabled)]
struct DetailRow {
    #[tabled(rename = "步骤")]
    step: String,
    #[tabled(rename = "来源")]
    source: String,
    #[tabled(rename = "抵扣")]
    amount: String,
    #[tabled(rename = "说明")]
    description: String,
}

fn step_name(step: &str) -> String {
    match step {
        "coupon" => "优惠券".to_string(),
        "voucher" => "代金券".to_string(),
        "balance" => "余额".to_string(),
        other => other.to_string(),
    }
}

/// 订单与结算明细（对齐 SettleDetailPanel：逐项列出抵扣）
fn print_order(ctx: &Ctx, order: &Order) {
    println!("订单\t{}（{}）", order.id, order.subject);
    println!("账户\t{}", order.account_id);
    println!("金额\t{}", money::format_cents(order.amount_cents, false));
    println!("状态\t{}", status::status_chip(&order.status, ctx.color));
    let rows: Vec<DetailRow> = order
        .settle_details
        .iter()
        .map(|d| DetailRow {
            step: step_name(&d.step),
            source: d.source_id.clone(),
            amount: money::format_cents_colored(d.amount_cents, true, ctx.color),
            description: d.description.clone(),
        })
        .collect();
    println!("抵扣明细（{} 项）", rows.len());
    println!("{}", output::table(rows));
}
