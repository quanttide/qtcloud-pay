use serde::{Deserialize, Serialize};

/// 账户（客户虚拟钱包），余额为整数分。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct Account {
    pub id: String,
    pub name: String,
    pub balance_cents: i64,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct CreateAccountRequest {
    pub name: String,
}
