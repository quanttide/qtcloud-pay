use std::path::PathBuf;

use serde::Deserialize;

use crate::error::CliError;

/// 配置文件（~/.config/qtcloud-pay/config.toml）
#[derive(Debug, Clone, Deserialize, Default)]
struct FileConfig {
    server_url: Option<String>,
}

#[derive(Debug, Clone)]
pub struct Config {
    pub server_url: String,
}

const ENV_SERVER_URL: &str = "QTPAY_SERVER_URL";
const DEFAULT_SERVER_URL: &str = "http://localhost:8080";

impl Config {
    /// 配置来源优先级：--server 参数 > QTPAY_SERVER_URL > config.toml > 默认值。
    pub fn load(server_flag: Option<&str>) -> Result<Config, CliError> {
        if let Some(url) = server_flag {
            return Ok(Config { server_url: normalize(url) });
        }
        if let Ok(url) = std::env::var(ENV_SERVER_URL) {
            return Ok(Config { server_url: normalize(&url) });
        }
        if let Some(path) = config_file_path() {
            if path.exists() {
                let content = std::fs::read_to_string(&path)
                    .map_err(|e| CliError::Fatal(format!("读取配置 {} 失败：{e}", path.display())))?;
                let file: FileConfig = toml::from_str(&content)
                    .map_err(|e| CliError::Fatal(format!("解析配置 {} 失败：{e}", path.display())))?;
                if let Some(url) = file.server_url {
                    return Ok(Config { server_url: normalize(&url) });
                }
            }
        }
        Ok(Config { server_url: DEFAULT_SERVER_URL.to_string() })
    }

    /// 里程碑验收本地登记文件路径（~/.config/qtcloud-pay/state.toml）
    pub fn state_file_path() -> Option<PathBuf> {
        home_dir().map(|home| home.join(".config/qtcloud-pay/state.toml"))
    }
}

fn normalize(url: &str) -> String {
    url.trim().trim_end_matches('/').to_string()
}

fn home_dir() -> Option<PathBuf> {
    std::env::var_os("HOME").map(PathBuf::from)
}

fn config_file_path() -> Option<PathBuf> {
    home_dir().map(|home| home.join(".config/qtcloud-pay/config.toml"))
}
