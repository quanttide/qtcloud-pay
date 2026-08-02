use crate::api::{parse, ApiError, Client};
use crate::models::coupon::{Coupon, IssueCouponRequest, IssueCouponResult};
use crate::models::voucher::{IssueVoucherRequest, IssueVoucherResult, Voucher};

impl Client {
    /// POST /accounts/{id}/coupons 发放优惠券（幂等键 = 发放批次号）
    pub async fn issue_coupon(
        &self,
        account_id: &str,
        req: &IssueCouponRequest,
    ) -> Result<IssueCouponResult, ApiError> {
        let resp = self
            .http
            .post(self.url(&format!("/accounts/{account_id}/coupons")))
            .json(req)
            .send()
            .await?;
        parse(resp).await
    }

    /// GET /accounts/{id}/coupons 查询优惠券
    pub async fn list_coupons(&self, account_id: &str) -> Result<Vec<Coupon>, ApiError> {
        let resp = self
            .http
            .get(self.url(&format!("/accounts/{account_id}/coupons")))
            .send()
            .await?;
        parse(resp).await
    }

    /// POST /accounts/{id}/vouchers 发放代金券（幂等键 = 发放批次号）
    pub async fn issue_voucher(
        &self,
        account_id: &str,
        req: &IssueVoucherRequest,
    ) -> Result<IssueVoucherResult, ApiError> {
        let resp = self
            .http
            .post(self.url(&format!("/accounts/{account_id}/vouchers")))
            .json(req)
            .send()
            .await?;
        parse(resp).await
    }

    /// GET /accounts/{id}/vouchers 查询代金券
    pub async fn list_vouchers(&self, account_id: &str) -> Result<Vec<Voucher>, ApiError> {
        let resp = self
            .http
            .get(self.url(&format!("/accounts/{account_id}/vouchers")))
            .send()
            .await?;
        parse(resp).await
    }
}
