use crate::api::ApiError;

/// CLI 错误，按退出码分类：1 业务错误 / 2 用法错误 / 3 网络与服务端错误。
#[derive(Debug, thiserror::Error)]
pub enum CliError {
    /// 业务错误（幂等冲突、过期券、对账差异等），退出码 1
    #[error("{0}")]
    Business(String),
    /// 用法错误（参数缺失、金额格式非法等），退出码 2
    #[error("用法错误：{0}")]
    Usage(String),
    /// 网络/服务端/内部错误，退出码 3
    #[error("{0}")]
    Fatal(String),
}

impl CliError {
    pub fn exit_code(&self) -> u8 {
        match self {
            CliError::Business(_) => 1,
            CliError::Usage(_) => 2,
            CliError::Fatal(_) => 3,
        }
    }
}

impl From<ApiError> for CliError {
    fn from(e: ApiError) -> Self {
        match e {
            ApiError::Business { message, .. } => CliError::Business(message),
            other => CliError::Fatal(other.to_string()),
        }
    }
}
