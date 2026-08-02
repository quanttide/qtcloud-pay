use colored::Colorize;

/// 解析元（如 "100.00"、"20"、"20.5"）为整数分；非负、最多两位小数。
/// 与领域模型约定一致：金额一律整数分存储，避免浮点误差。
pub fn parse_yuan_to_cents(input: &str) -> Result<i64, String> {
    let s = input.trim();
    if s.is_empty() {
        return Err("金额不能为空".to_string());
    }
    let (int_part, frac_part) = match s.split_once('.') {
        Some((i, f)) => (i, f),
        None => (s, ""),
    };
    if int_part.is_empty() || !int_part.chars().all(|c| c.is_ascii_digit()) {
        return Err(format!(
            "金额格式非法：{input}（应为非负数字，最多两位小数）"
        ));
    }
    if frac_part.len() > 2 || !frac_part.chars().all(|c| c.is_ascii_digit()) {
        return Err(format!("金额格式非法：{input}（最多两位小数）"));
    }
    let yuan: i64 = int_part
        .parse()
        .map_err(|_| format!("金额超出范围：{input}"))?;
    let cents: i64 = match frac_part.len() {
        0 => 0,
        1 => {
            frac_part
                .parse::<i64>()
                .map_err(|_| format!("金额超出范围：{input}"))?
                * 10
        }
        2 => frac_part
            .parse::<i64>()
            .map_err(|_| format!("金额超出范围：{input}"))?,
        _ => unreachable!(),
    };
    Ok(yuan * 100 + cents)
}

/// 分 → 元字符串（无浮点误差）：10000 → "100.00"。
/// signed 时正数带 "+"、负数带 "-"。
pub fn format_cents(cents: i64, signed: bool) -> String {
    let sign = if signed {
        if cents > 0 {
            "+"
        } else if cents < 0 {
            "-"
        } else {
            ""
        }
    } else {
        ""
    };
    let abs = cents.abs();
    format!("{sign}{}.{:02}", abs / 100, abs % 100)
}

/// 着色版本（MoneyText 对应物）：正数绿、负数红；color=false 时仅符号。
pub fn format_cents_colored(cents: i64, signed: bool, color: bool) -> String {
    let text = format_cents(cents, signed);
    if !color {
        return text;
    }
    if cents > 0 {
        text.green().to_string()
    } else if cents < 0 {
        text.red().to_string()
    } else {
        text
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_basic() {
        assert_eq!(parse_yuan_to_cents("100.00").unwrap(), 10000);
        assert_eq!(parse_yuan_to_cents("100").unwrap(), 10000);
        assert_eq!(parse_yuan_to_cents("0.01").unwrap(), 1);
        assert_eq!(parse_yuan_to_cents("20.5").unwrap(), 2050);
        assert_eq!(parse_yuan_to_cents(" 10.00 ").unwrap(), 1000);
    }

    #[test]
    fn parse_invalid() {
        for bad in ["", "abc", "-5", "1.234", "1..2", "12.3.4"] {
            assert!(parse_yuan_to_cents(bad).is_err(), "应拒绝：{bad}");
        }
    }

    #[test]
    fn format_basic() {
        assert_eq!(format_cents(10000, false), "100.00");
        assert_eq!(format_cents(10000, true), "+100.00");
        assert_eq!(format_cents(-2000, true), "-20.00");
        assert_eq!(format_cents(0, true), "0.00");
    }
}
