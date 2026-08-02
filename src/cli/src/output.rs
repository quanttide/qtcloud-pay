use serde::Serialize;
use tabled::settings::Style;
use tabled::{Table, Tabled};

use crate::error::CliError;

/// 表格渲染（默认输出）。
pub fn table<T: Tabled>(rows: Vec<T>) -> String {
    Table::new(rows).with(Style::modern()).to_string()
}

/// JSON 渲染（--json，金额保持整数分，供脚本消费）。
pub fn print_json<T: Serialize>(value: &T) -> Result<(), CliError> {
    let json = serde_json::to_string_pretty(value)
        .map_err(|e| CliError::Fatal(format!("JSON 序列化失败：{e}")))?;
    println!("{json}");
    Ok(())
}
