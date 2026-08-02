use serde::{Deserialize, Serialize};

/// 计费抵扣顺序（BillingRule.priority），v0.1.0 默认：优惠券 → 代金券 → 余额。
/// 服务端参数 API 就绪后由配置下发；当前为设计占位。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct BillingRule {
    pub priority: Vec<String>,
}

impl Default for BillingRule {
    fn default() -> Self {
        BillingRule {
            priority: vec!["coupon".into(), "voucher".into(), "balance".into()],
        }
    }
}
