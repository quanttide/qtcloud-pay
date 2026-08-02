use serde::{Deserialize, Serialize};

use crate::models::account::Account;

/// 订单：客户购买付费服务的交易请求
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct Order {
    pub id: String,
    pub account_id: String,
    pub subject: String,
    pub amount_cents: i64,
    /// settled 已结算 / pending 待结算
    pub status: String,
    pub settle_details: Vec<SettleDetail>,
    pub created_at: String,
}

/// 逐项抵扣明细（优惠券 → 代金券 → 余额）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct SettleDetail {
    /// coupon 优惠券 / voucher 代金券 / balance 余额
    pub step: String,
    pub source_id: String,
    /// 抵扣金额（负数）
    pub amount_cents: i64,
    pub description: String,
}

/// 下单并结算请求（幂等键 = 订单号）
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct CreateOrderRequest {
    pub account_id: String,
    pub amount_cents: i64,
    pub order_no: String,
    pub subject: Option<String>,
}

/// 下单结算结果：订单 + 更新后账户
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct CreateOrderResult {
    pub order: Order,
    pub account: Account,
}
