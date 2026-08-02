use crate::args::{RechargesAction, RechargesArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::transaction::RechargeRequest;
use crate::money;
use crate::output;

pub async fn run(ctx: &Ctx, args: RechargesArgs) -> Result<(), CliError> {
    match args.action {
        RechargesAction::Create { account_id, amount, receipt_no } => {
            create(ctx, &account_id, &amount, &receipt_no).await
        }
    }
}

/// POST /accounts/{id}/recharges 充值登记（幂等键 = 打款凭证号）
async fn create(ctx: &Ctx, account_id: &str, amount: &str, receipt_no: &str) -> Result<(), CliError> {
    let amount_cents = money::parse_yuan_to_cents(amount).map_err(CliError::Usage)?;
    if amount_cents <= 0 {
        return Err(CliError::Usage("充值金额必须大于 0".to_string()));
    }
    if receipt_no.trim().is_empty() {
        return Err(CliError::Usage("打款凭证号（--receipt-no）必填，作为幂等键".to_string()));
    }
    let result = ctx
        .client
        .create_recharge(
            account_id,
            &RechargeRequest { amount_cents, receipt_no: receipt_no.trim().to_string() },
        )
        .await?;
    if ctx.json {
        return output::print_json(&result);
    }
    println!("✔ 充值成功（凭证号 {}）", receipt_no.trim());
    println!("账户\t{}", result.account.id);
    println!("金额\t{}", money::format_cents_colored(result.transaction.amount_cents, true, ctx.color));
    println!("交易\t{}", result.transaction.id);
    println!("余额\t{}", money::format_cents_colored(result.account.balance_cents, true, ctx.color));
    Ok(())
}
