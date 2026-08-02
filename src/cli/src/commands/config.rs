use crate::args::{BillingRuleAction, ConfigAction, ConfigArgs};
use crate::commands::Ctx;
use crate::error::CliError;
use crate::models::billing_rule::BillingRule;

pub async fn run(ctx: &Ctx, args: ConfigArgs) -> Result<(), CliError> {
    match args.action {
        ConfigAction::BillingRule(action) => match action {
            BillingRuleAction::Show => show(ctx).await,
            BillingRuleAction::Set { priority } => set(ctx, &priority).await,
        },
    }
}

/// 展示默认抵扣顺序（BillingRule.priority）
async fn show(_ctx: &Ctx) -> Result<(), CliError> {
    let rule = BillingRule::default();
    println!("计费抵扣顺序（BillingRule.priority）：{}", rule.priority.join(" → "));
    println!("1. 满足条件的优惠券（满减先减、折扣按比例）");
    println!("2. 代金券抵现");
    println!("3. 余额补足");
    println!("变更登记：在 studio.md §3.4 参数变更登记表登记（申请人/审批）");
    Ok(())
}

/// 设置抵扣顺序（预留：服务端参数 API 就绪后启用）
async fn set(_ctx: &Ctx, priority: &str) -> Result<(), CliError> {
    println!("预留命令：服务端参数 API 就绪后启用，抵扣顺序由 BillingRule.priority 配置，不改代码可调。");
    println!("收到优先级：{priority}");
    Ok(())
}
