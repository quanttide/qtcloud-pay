use std::collections::BTreeMap;

use chrono::Local;
use colored::Colorize;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::api::ApiError;
use crate::args::{MilestoneAction, MilestoneArgs};
use crate::commands::Ctx;
use crate::config::Config;
use crate::error::CliError;
use crate::models::transaction::RechargeRequest;

/// M1–M5 里程碑（对齐 provider.md）
pub(crate) const MILESTONES: [(&str, &str); 5] = [
    ("M1", "账户与账本"),
    ("M2", "优惠券与代金券"),
    ("M3", "订单与计费规则"),
    ("M4", "对账与可查"),
    ("M5", "支付通道对接（v0.2.0）"),
];

/// 本地验收登记（~/.config/qtcloud-pay/state.toml，仅本地记录，不修改服务端）
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub(crate) struct AcceptanceEntry {
    pub date: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct StateFile {
    #[serde(default)]
    acceptance: BTreeMap<String, AcceptanceEntry>,
}

pub(crate) fn load_acceptance() -> BTreeMap<String, AcceptanceEntry> {
    let Some(path) = Config::state_file_path() else {
        return BTreeMap::new();
    };
    let Ok(content) = std::fs::read_to_string(path) else {
        return BTreeMap::new();
    };
    toml::from_str::<StateFile>(&content)
        .map(|s| s.acceptance)
        .unwrap_or_default()
}

fn save_acceptance(map: &BTreeMap<String, AcceptanceEntry>) -> Result<(), CliError> {
    let Some(path) = Config::state_file_path() else {
        return Err(CliError::Fatal("无法确定主目录（HOME 未设置）".to_string()));
    };
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| CliError::Fatal(format!("创建目录 {} 失败：{e}", parent.display())))?;
    }
    let content = toml::to_string(&StateFile {
        acceptance: map.clone(),
    })
    .map_err(|e| CliError::Fatal(format!("序列化验收登记失败：{e}")))?;
    std::fs::write(&path, content)
        .map_err(|e| CliError::Fatal(format!("写入 {} 失败：{e}", path.display())))?;
    Ok(())
}

pub async fn run(ctx: &Ctx, args: MilestoneArgs) -> Result<(), CliError> {
    match args.action {
        MilestoneAction::List => list(),
        MilestoneAction::Check { milestone, note } => check(&milestone, note.as_deref()),
        MilestoneAction::Verify { milestone } => verify(ctx, &milestone).await,
    }
}

/// 里程碑列表（输出对齐 studio.md §一 仪表盘）
fn list() -> Result<(), CliError> {
    let acceptance = load_acceptance();
    println!("里程碑状态（本地登记）：");
    for (id, name) in MILESTONES {
        let mark = match acceptance.get(id) {
            Some(entry) => format!(
                "✅ 已完成（{}）{}",
                entry.date,
                entry
                    .note
                    .as_deref()
                    .map(|n| format!(" — {n}"))
                    .unwrap_or_default()
            ),
            None => "⬜ 未开始".to_string(),
        };
        if id == "M5" && !acceptance.contains_key(id) {
            println!("{id}\t{name}\t{mark}（依赖 M1–M4 完成）");
        } else {
            println!("{id}\t{name}\t{mark}");
        }
    }
    Ok(())
}

/// 验收打勾并记录日期
fn check(milestone: &str, note: Option<&str>) -> Result<(), CliError> {
    validate_milestone(milestone)?;
    let mut acceptance = load_acceptance();
    let today = Local::now().format("%Y-%m-%d").to_string();
    acceptance.insert(
        milestone.to_string(),
        AcceptanceEntry {
            date: today.clone(),
            note: note.map(str::to_string),
        },
    );
    save_acceptance(&acceptance)?;
    println!("✔ {milestone} 已登记验收（{today}）");
    Ok(())
}

/// 自动执行验收。当前支持 M1（创建账户 → 充值 → 重复登记幂等拦截 → 余额校验）；
/// M2–M4 使用对应命令手动验收（见 data/roadmap/cli.md §八）。
async fn verify(ctx: &Ctx, milestone: &str) -> Result<(), CliError> {
    validate_milestone(milestone)?;
    if milestone != "M1" {
        return Err(CliError::Usage(format!(
            "milestone verify 当前支持 M1 自动验收；{milestone} 使用对应命令手动验收（见 data/roadmap/cli.md §八）"
        )));
    }
    let mut failed = 0usize;

    // 1. 创建账户
    let account = match ctx
        .client
        .create_account(&format!("验收-M1-{}", Uuid::new_v4().simple()))
        .await
    {
        Ok(a) => {
            step(1, "创建账户", true, "");
            a
        }
        Err(e) => {
            step(1, "创建账户", false, &e.to_string());
            failed += 1;
            return summary(failed);
        }
    };

    // 2. 充值登记（100.00 元）
    let receipt = format!("accept-{}", Uuid::new_v4().simple());
    let recharge_req = RechargeRequest {
        amount_cents: 10000,
        receipt_no: receipt.clone(),
    };
    match ctx.client.create_recharge(&account.id, &recharge_req).await {
        Ok(_) => step(2, "充值登记（100.00 元）", true, ""),
        Err(e) => {
            step(2, "充值登记（100.00 元）", false, &e.to_string());
            failed += 1;
        }
    }

    // 3. 重复登记同一凭证号 → 幂等拦截（不重）
    match ctx.client.create_recharge(&account.id, &recharge_req).await {
        Err(ApiError::Business { .. }) => step(3, "重复登记幂等拦截", true, "幂等键唯一约束生效"),
        Ok(_) => {
            step(3, "重复登记幂等拦截", false, "重复登记未被拦截");
            failed += 1;
        }
        Err(e) => {
            step(3, "重复登记幂等拦截", false, &e.to_string());
            failed += 1;
        }
    }

    // 4. 余额 = 100.00 元（不错）
    match ctx.client.get_account(&account.id).await {
        Ok(a) if a.balance_cents == 10000 => step(4, "余额校验（100.00 元）", true, ""),
        Ok(a) => {
            step(
                4,
                "余额校验（100.00 元）",
                false,
                &format!("余额为 {} 分", a.balance_cents),
            );
            failed += 1;
        }
        Err(e) => {
            step(4, "余额校验（100.00 元）", false, &e.to_string());
            failed += 1;
        }
    }

    // 5. 交易流水可查（可查）
    match ctx.client.list_transactions(&account.id).await {
        Ok(txns) if txns.iter().filter(|t| t.kind == "recharge").count() == 1 => {
            step(5, "交易流水（1 笔充值）", true, "")
        }
        Ok(txns) => {
            step(
                5,
                "交易流水（1 笔充值）",
                false,
                &format!("共 {} 笔交易", txns.len()),
            );
            failed += 1;
        }
        Err(e) => {
            step(5, "交易流水（1 笔充值）", false, &e.to_string());
            failed += 1;
        }
    }

    summary(failed)
}

fn validate_milestone(milestone: &str) -> Result<(), CliError> {
    if MILESTONES.iter().any(|(id, _)| *id == milestone) {
        Ok(())
    } else {
        Err(CliError::Usage(format!(
            "里程碑非法：{milestone}（支持 M1–M5）"
        )))
    }
}

fn step(n: usize, name: &str, ok: bool, detail: &str) {
    let mark = if ok {
        "✔".green().to_string()
    } else {
        "✘".red().to_string()
    };
    if detail.is_empty() {
        println!("[{n}] {name}\t{mark}");
    } else {
        println!("[{n}] {name}\t{mark} {detail}");
    }
}

fn summary(failed: usize) -> Result<(), CliError> {
    if failed == 0 {
        println!();
        println!("✔ M1 验收通过（测试数据已写入，建议在测试环境执行）");
        Ok(())
    } else {
        Err(CliError::Business(format!(
            "M1 验收未通过：{failed} 项失败，请检查服务端与数据后重试"
        )))
    }
}
