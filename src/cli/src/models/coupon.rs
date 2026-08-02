use serde::{Deserialize, Serialize};

use crate::models::transaction::Transaction;

/// 优惠券：折扣券（rate）/ 满减券（threshold_cents + amount_cents）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct Coupon {
    pub id: String,
    pub account_id: String,
    /// rate 折扣券 / threshold 满减券
    pub kind: String,
    pub rate: Option<f64>,
    pub threshold_cents: Option<i64>,
    pub amount_cents: Option<i64>,
    pub scope: String,
    pub expires_at: String,
    /// issued 已发放 / used 已使用 / expired 已过期
    pub status: String,
}

/// 发放优惠券请求（幂等键 = 发放批次号）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct IssueCouponRequest {
    pub kind: String,
    pub rate: Option<f64>,
    pub threshold_cents: Option<i64>,
    pub amount_cents: Option<i64>,
    pub scope: Option<String>,
    pub expires_at: Option<String>,
    pub batch_no: String,
}

/// 发券结果：券 + 发券交易（账本完整，不丢）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct IssueCouponResult {
    pub coupon: Coupon,
    pub transaction: Transaction,
}
