//! qtcloud-pay CLI：量潮支付账本核心命令行工作台。
//!
//! 设计文档见 `data/roadmap/cli.md`。CLI 只做展示与参数组装，账务逻辑（结算、幂等、状态机）全在服务端。

pub mod api;
pub mod args;
pub mod commands;
pub mod config;
pub mod error;
pub mod models;
pub mod money;
pub mod output;
pub mod reconcile;
pub mod status;
