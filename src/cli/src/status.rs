use colored::Colorize;

/// 状态标签（StatusChip 对应物）。
/// 券：已发放 / 已使用 / 已过期；订单：已结算 / 待结算。
/// 服务端状态码与中文显示名均支持；未知状态原样输出。
pub fn status_chip(raw: &str, color: bool) -> String {
    let display = match raw {
        "issued" | "已发放" => "已发放",
        "used" | "已使用" => "已使用",
        "expired" | "已过期" => "已过期",
        "settled" | "已结算" => "已结算",
        "pending" | "待结算" => "待结算",
        other => other,
    };
    if !color {
        return display.to_string();
    }
    match raw {
        "issued" | "已发放" | "settled" | "已结算" => display.green().to_string(),
        "used" | "已使用" => display.blue().to_string(),
        "expired" | "已过期" => display.red().to_string(),
        "pending" | "待结算" => display.yellow().to_string(),
        _ => display.to_string(),
    }
}

/// 交易类型显示名：recharge 充值 / consume 消费 / issue 发券 / redeem 核销
pub fn transaction_kind(raw: &str) -> String {
    match raw {
        "recharge" => "充值".to_string(),
        "consume" => "消费".to_string(),
        "issue" => "发券".to_string(),
        "redeem" => "核销".to_string(),
        other => other.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn chip_maps_codes_and_zh() {
        assert_eq!(status_chip("issued", false), "已发放");
        assert_eq!(status_chip("已发放", false), "已发放");
        assert_eq!(status_chip("expired", false), "已过期");
        assert_eq!(status_chip("settled", false), "已结算");
        assert_eq!(status_chip("pending", false), "待结算");
        assert_eq!(status_chip("unknown", false), "unknown");
    }

    #[test]
    fn kind_maps_codes() {
        assert_eq!(transaction_kind("recharge"), "充值");
        assert_eq!(transaction_kind("consume"), "消费");
        assert_eq!(transaction_kind("issue"), "发券");
        assert_eq!(transaction_kind("redeem"), "核销");
        assert_eq!(transaction_kind("other"), "other");
    }
}
