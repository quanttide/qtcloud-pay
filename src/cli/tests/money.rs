use qtcloud_pay_cli::money::{format_cents, parse_yuan_to_cents};

#[test]
fn parse_yuan() {
    assert_eq!(parse_yuan_to_cents("100.00").unwrap(), 10000);
    assert_eq!(parse_yuan_to_cents("100").unwrap(), 10000);
    assert_eq!(parse_yuan_to_cents("0.01").unwrap(), 1);
    assert_eq!(parse_yuan_to_cents("20.5").unwrap(), 2050);
}

#[test]
fn parse_rejects_invalid() {
    for bad in ["", "abc", "-5", "1.234", "1..2", "12.3.4"] {
        assert!(parse_yuan_to_cents(bad).is_err(), "应拒绝：{bad}");
    }
}

#[test]
fn format_yuan() {
    assert_eq!(format_cents(10000, false), "100.00");
    assert_eq!(format_cents(10000, true), "+100.00");
    assert_eq!(format_cents(-2000, true), "-20.00");
    assert_eq!(format_cents(0, true), "0.00");
}
