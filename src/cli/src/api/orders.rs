use crate::api::{parse, ApiError, Client};
use crate::models::order::{CreateOrderRequest, CreateOrderResult, Order};

impl Client {
    /// POST /orders 下单并结算（幂等键 = 订单号）
    pub async fn create_order(&self, req: &CreateOrderRequest) -> Result<CreateOrderResult, ApiError> {
        let resp = self.http.post(self.url("/orders")).json(req).send().await?;
        parse(resp).await
    }

    /// GET /orders/{id} 订单与结算明细
    pub async fn get_order(&self, order_id: &str) -> Result<Order, ApiError> {
        let resp = self.http.get(self.url(&format!("/orders/{order_id}"))).send().await?;
        parse(resp).await
    }
}
