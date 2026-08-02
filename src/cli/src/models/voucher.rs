use serde::{Deserialize, Serialize};

use crate::models::transaction::Transaction;

/// 代金券：固定面值，结算直接抵现
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct Voucher {
    pub id: String,
    pub account_id: String,
    pub amount_cents: i64,
    pub scope: String,
    pub expires_at: String,
    /// issued 已发放 / used 已使用 / expired 已过期
    pub status: String,
}

/// 发放代金券请求（幂等键 = 发放批次号）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct IssueVoucherRequest {
    pub amount_cents: i64,
    pub scope: Option<String>,
    pub expires_at: Option<String>,
    pub batch_no: String,
}

/// 发券结果：代金券 + 发券交易
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct IssueVoucherResult {
    pub voucher: Voucher,
    pub transaction: Transaction,
}
