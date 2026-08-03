// Package money 金额表示与 JSON 传输约定。
//
// 内部一律以分（int64）参与计算（账本、余额、交易、券、订单）；
// API 边界以元（两位小数数字，如 99.99）传输，符合人的直觉。
// Cents 类型负责边界转换：MarshalJSON 分 → 元，UnmarshalJSON 元 → 分（严格校验两位小数）。
package money

import (
	"fmt"
	"strconv"
	"strings"
)

// Cents 金额（分）。JSON 序列化为元（两位小数数字），反序列化接受元并校验两位小数。
type Cents int64

// MarshalJSON 分 → 元数字（整数运算，无浮点）：9999 → "99.99"。
func (c Cents) MarshalJSON() ([]byte, error) {
	v := int64(c)
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return []byte(fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)), nil
}

// UnmarshalJSON 元数字 → 分：99.99 → 9999。
// 纯整数字符串解析（不经浮点）：仅接受十进制数字，严格校验最多两位小数
// （拒绝 99.999），避免三位及以上小数被静默舍入入账；拒绝字符串与指数记法。
func (c *Cents) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "" || s[0] == '"' || s[0] == '\'' {
		return fmt.Errorf("money: invalid amount %s", s)
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	intPart, frac, hasDot := strings.Cut(s, ".")
	if hasDot && (len(frac) < 1 || len(frac) > 2) {
		return fmt.Errorf("money: amount must have at most 2 decimal places: %s", b)
	}
	whole, err1 := strconv.ParseInt(intPart, 10, 64)
	sub, err2 := strconv.ParseInt((frac+"00")[:2], 10, 64) // 小数位补零到两位
	if err1 != nil || err2 != nil {
		return fmt.Errorf("money: invalid amount %s", b)
	}
	v := whole*100 + sub
	if neg {
		v = -v
	}
	*c = Cents(v)
	return nil
}
