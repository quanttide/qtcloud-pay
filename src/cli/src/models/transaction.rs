use serde::{Deserialize, Serialize};

use crate::models::account::Account;

/// 交易（不可变记录）。amount_cents 带方向：充值 +、消费/核销 −、发券 0。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct Transaction {
    pub id: String,
    pub account_id: String,
    /// recharge 充值 / consume 消费 / issue 发券 / redeem 核销
    pub kind: String,
    pub amount_cents: i64,
    pub occurred_at: String,
    /// 来源：打款凭证号 / 发放批次号 / 订单号
    pub source: String,
}

/// 充值登记请求（幂等键 = 打款凭证号）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct RechargeRequest {
    pub amount_cents: i64,
    pub receipt_no: String,
}

/// 充值登记结果：更新后账户 + 新增充值交易
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct RechargeResult {
    pub account: Account,
    pub transaction: Transaction,
}
