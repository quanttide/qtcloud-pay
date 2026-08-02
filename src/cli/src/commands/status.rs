use crate::commands::milestone::{load_acceptance, MILESTONES};
use crate::commands::Ctx;
use crate::error::CliError;

/// 总览：服务端地址、里程碑状态与今日待办
pub async fn run(ctx: &Ctx) -> Result<(), CliError> {
    println!("账本核心工作台（qtcloud-pay CLI）");
    println!("服务端\t{}", ctx.server_url);
    println!();
    println!("里程碑状态（本地登记）：");
    let acceptance = load_acceptance();
    for (id, name) in MILESTONES {
        let mark = match acceptance.get(id) {
            Some(entry) => format!("✅ 已完成（{}）", entry.date),
            None => "⬜ 未开始".to_string(),
        };
        if id == "M5" && !acceptance.contains_key(id) {
            println!("{id}\t{name}\t{mark}（依赖 M1–M4 完成）");
        } else {
            println!("{id}\t{name}\t{mark}");
        }
    }
    println!();
    println!("今日待办：");
    println!("  - 对账：qtcloud-pay reconcile <账户>... --bank <银行流水.csv>");
    println!("  - 账单导出：qtcloud-pay accounts statement <账户> -o statement.csv");
    println!("  - 参数变更：qtcloud-pay config billing-rule show");
    Ok(())
}
