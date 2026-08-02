use serde::Serialize;
use tabled::Tabled;

use crate::args::{AccountsAction, AccountsArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::transaction::Transaction;
use crate::money;
use crate::output;
use crate::status;

pub async fn run(ctx: &Ctx, args: AccountsArgs) -> Result<(), CliError> {
    match args.action {
        AccountsAction::Create { name } => create(ctx, &name).await,
        AccountsAction::Get { account_id } => get(ctx, &account_id).await,
        AccountsAction::Transactions { account_id, type_ } => {
            transactions(ctx, &account_id, type_.as_deref()).await
        }
        AccountsAction::Statement { account_id, output } => {
            statement(ctx, &account_id, output.as_deref()).await
        }
    }
}

/// POST /accounts 创建账户
async fn create(ctx: &Ctx, name: &str) -> Result<(), CliError> {
    let account = ctx.client.create_account(name).await?;
    if ctx.json {
        return output::print_json(&account);
    }
    println!("✔ 账户已创建");
    println!("账户 id\t{}", account.id);
    println!("余额\t{}", money::format_cents_colored(account.balance_cents, true, ctx.color));
    Ok(())
}

/// GET /accounts/{id} 账户与余额
async fn get(ctx: &Ctx, account_id: &str) -> Result<(), CliError> {
    let account = ctx.client.get_account(account_id).await?;
    if ctx.json {
        return output::print_json(&account);
    }
    println!("账户 id\t{}", account.id);
    println!("客户\t{}", account.name);
    println!("余额\t{}", money::format_cents_colored(account.balance_cents, true, ctx.color));
    println!("创建时间\t{}", account.created_at);
    println!("提示\t余额应等于交易求和：qtcloud-pay reconcile {account_id}");
    Ok(())
}

#[derive(Debug, Clone, Serialize, Tabled)]
struct TransactionRow {
    #[tabled(rename = "类型")]
    kind: String,
    #[tabled(rename = "金额(分)")]
    amount: String,
    #[tabled(rename = "方向")]
    direction: String,
    #[tabled(rename = "时间")]
    occurred_at: String,
    #[tabled(rename = "来源")]
    source: String,
}

impl From<&Transaction> for TransactionRow {
    fn from(t: &Transaction) -> Self {
        TransactionRow {
            kind: status::transaction_kind(&t.kind),
            amount: format!("{:+}", t.amount_cents),
            direction: if t.amount_cents >= 0 { "+" } else { "-" }.to_string(),
            occurred_at: t.occurred_at.clone(),
            source: t.source.clone(),
        }
    }
}

/// GET /accounts/{id}/transactions 交易流水
async fn transactions(ctx: &Ctx, account_id: &str, kind_filter: Option<&str>) -> Result<(), CliError> {
    let txns = ctx.client.list_transactions(account_id).await?;
    let txns: Vec<&Transaction> = match kind_filter {
        Some(kind) => txns
            .iter()
            .filter(|t| t.kind == kind || status::transaction_kind(&t.kind) == kind)
            .collect(),
        None => txns.iter().collect(),
    };
    if ctx.json {
        return output::print_json(&txns);
    }
    let rows: Vec<TransactionRow> = txns.iter().map(|t| TransactionRow::from(*t)).collect();
    println!("账户 {account_id} 交易流水（{} 笔）", rows.len());
    println!("{}", output::table(rows));
    Ok(())
}

/// GET /accounts/{id}/statement 账单导出（CSV）
async fn statement(ctx: &Ctx, account_id: &str, output_path: Option<&str>) -> Result<(), CliError> {
    let body = ctx.client.statement(account_id).await?;
    match output_path {
        Some(path) => {
            std::fs::write(path, &body)
                .map_err(|e| CliError::Fatal(format!("写入 {path} 失败：{e}")))?;
            if !ctx.quiet {
                println!("✔ 账单已导出：{path}");
            }
        }
        None => print!("{body}"),
    }
    Ok(())
}
