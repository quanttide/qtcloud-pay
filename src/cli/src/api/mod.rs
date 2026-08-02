pub mod accounts;
pub mod coupons;
pub mod error;
pub mod orders;

pub use error::ApiError;

use serde::de::DeserializeOwned;
use serde::Deserialize;

#[derive(Debug, Clone)]
pub struct Client {
    pub(crate) http: reqwest::Client,
    pub(crate) base_url: String,
}

impl Client {
    pub fn new(base_url: String) -> Result<Client, ApiError> {
        let http = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()?;
        Ok(Client { http, base_url })
    }

    pub(crate) fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}

#[derive(Debug, Deserialize)]
struct ErrorBody {
    code: Option<String>,
    message: Option<String>,
}

/// 发送请求并解析响应；4xx 归为业务错误，5xx 归为服务端错误。
pub(crate) async fn parse<T: DeserializeOwned>(resp: reqwest::Response) -> Result<T, ApiError> {
    let status = resp.status();
    let body = resp.text().await?;
    if !status.is_success() {
        let err: ErrorBody = serde_json::from_str(&body).unwrap_or(ErrorBody {
            code: None,
            message: None,
        });
        if status.is_client_error() {
            return Err(ApiError::Business {
                code: err.code.unwrap_or_else(|| status.to_string()),
                message: err.message.unwrap_or(body),
            });
        }
        return Err(ApiError::Server {
            status: status.as_u16(),
            body,
        });
    }
    serde_json::from_str(&body).map_err(ApiError::Decode)
}
