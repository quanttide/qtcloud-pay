// Package money 金额表示与 JSON 传输约定。
//
// 内部一律以分（int64）参与计算（账本、余额、交易、券、订单）；
// API 边界以元（两位小数数字，如 99.99）传输，符合人的直觉。
// Cents 类型负责边界转换：MarshalJSON 分 → 元，UnmarshalJSON 元 → 分（严格校验两位小数）。
package money

import (
	"encoding/json"
	"fmt"
	"math"
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
// 严格校验最多两位小数（拒绝 99.999），避免三位及以上小数被静默舍入入账。
func (c *Cents) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("money: invalid amount %s", b)
	}
	cents := math.Round(f * 100)
	if math.Abs(f*100-cents) > 1e-6 {
		return fmt.Errorf("money: amount must have at most 2 decimal places: %s", b)
	}
	*c = Cents(int64(cents))
	return nil
}
