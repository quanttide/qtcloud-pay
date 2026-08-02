/// API 错误：业务错误（退出码 1）与网络/服务端错误（退出码 3）分开，便于脚本处置。
#[derive(Debug, thiserror::Error)]
pub enum ApiError {
    #[error("请求失败：{0}")]
    Network(#[from] reqwest::Error),
    #[error("服务端错误 {status}：{body}")]
    Server { status: u16, body: String },
    /// 业务错误（幂等冲突、过期券等），code 供脚本判断
    #[error("{message}")]
    Business { code: String, message: String },
    #[error("响应解析失败：{0}")]
    Decode(#[from] serde_json::Error),
}

impl ApiError {
    pub fn code(&self) -> Option<&str> {
        match self {
            ApiError::Business { code, .. } => Some(code),
            _ => None,
        }
    }
}
