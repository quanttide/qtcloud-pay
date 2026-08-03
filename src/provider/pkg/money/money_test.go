package money

import (
	"encoding/json"
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	cases := []struct {
		cents Cents
		want  string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{100, "1.00"},
		{9999, "99.99"},
		{19900, "199.00"},
		{-150, "-1.50"},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.cents)
		if err != nil {
			t.Fatalf("Marshal(%d): %v", c.cents, err)
		}
		if string(b) != c.want {
			t.Errorf("Marshal(%d) = %s, want %s", c.cents, b, c.want)
		}
	}
}

func TestUnmarshalJSON(t *testing.T) {
	cases := []struct {
		in   string
		want Cents
		ok   bool
	}{
		{"99.99", 9999, true},
		{"100", 10000, true},
		{"0.1", 10, true},
		{"0.29", 29, true}, // 浮点边界：0.29 × 100 舍入
		{"199.00", 19900, true},
		{"99.999", 0, false}, // 三位小数拒绝
		{"0.001", 0, false},
		{"1.", 0, false},     // 尾点拒绝（JSON 数字语法）
		{"1e3", 0, false},    // 指数记法拒绝（金额仅十进制）
		{"+1.00", 0, false},  // 前导加号拒绝（JSON 数字语法）
		{"-1.50", -150, true},
		{"-0.05", -5, true},
		{`"99.99"`, 0, false}, // 字符串拒绝（API 约定为数字）
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		var got Cents
		err := json.Unmarshal([]byte(c.in), &got)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("Unmarshal(%s) = %d, %v; want %d", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("Unmarshal(%s) should error, got %d", c.in, got)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, cents := range []Cents{0, 1, 9999, 1000000, 123456789, -150, -123456789} {
		b, err := json.Marshal(cents)
		if err != nil {
			t.Fatal(err)
		}
		var got Cents
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if got != cents {
			t.Errorf("round trip %d → %s → %d", cents, b, got)
		}
	}
}
