use crate::api::{parse, ApiError, Client};
use crate::models::account::{Account, CreateAccountRequest};
use crate::models::transaction::{RechargeRequest, RechargeResult, Transaction};

impl Client {
    /// POST /accounts 创建账户
    pub async fn create_account(&self, name: &str) -> Result<Account, ApiError> {
        let resp = self
            .http
            .post(self.url("/accounts"))
            .json(&CreateAccountRequest {
                name: name.to_string(),
            })
            .send()
            .await?;
        parse(resp).await
    }

    /// GET /accounts/{id} 账户与余额
    pub async fn get_account(&self, id: &str) -> Result<Account, ApiError> {
        let resp = self
            .http
            .get(self.url(&format!("/accounts/{id}")))
            .send()
            .await?;
        parse(resp).await
    }

    /// GET /accounts/{id}/transactions 交易流水
    pub async fn list_transactions(&self, id: &str) -> Result<Vec<Transaction>, ApiError> {
        let resp = self
            .http
            .get(self.url(&format!("/accounts/{id}/transactions")))
            .send()
            .await?;
        parse(resp).await
    }

    /// GET /accounts/{id}/statement 账单导出（CSV 文本）
    pub async fn statement(&self, id: &str) -> Result<String, ApiError> {
        let resp = self
            .http
            .get(self.url(&format!("/accounts/{id}/statement")))
            .send()
            .await?;
        let status = resp.status();
        let body = resp.text().await?;
        if !status.is_success() {
            if status.is_client_error() {
                return Err(ApiError::Business {
                    code: status.to_string(),
                    message: body,
                });
            }
            return Err(ApiError::Server {
                status: status.as_u16(),
                body,
            });
        }
        Ok(body)
    }

    /// POST /accounts/{id}/recharges 充值登记（幂等键 = 打款凭证号）
    pub async fn create_recharge(
        &self,
        account_id: &str,
        req: &RechargeRequest,
    ) -> Result<RechargeResult, ApiError> {
        let resp = self
            .http
            .post(self.url(&format!("/accounts/{account_id}/recharges")))
            .json(req)
            .send()
            .await?;
        parse(resp).await
    }
}
