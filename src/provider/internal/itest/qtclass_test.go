package itest

import (
	"net/http"
	"slices"
	"testing"
	"time"
)

// futureExpiry 明天的过期时间（RFC3339）。
func futureExpiry() string {
	return time.Now().Add(24 * time.Hour).Format(time.RFC3339)
}

// TC-A01 开户与付费记额度：余额正确；重复提交同凭证号不重复入账（不重）。
func TestA01_RechargeIdempotent(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")

	e.recharge(acc, 20000, "GT-001")
	e.recharge(acc, 20000, "GT-001") // 重复提交

	e.assertLedger(acc, 20000)
	txs := e.transactions(acc)
	if len(txs) != 1 {
		t.Fatalf("txs = %d, want 1", len(txs))
	}
	tx := txs[0]
	if tx["type"] != "recharge" || centsOf(tx["amount"]) != 20000 ||
		centsOf(tx["balance_after"]) != 20000 {
		t.Errorf("tx = %v", tx)
	}
}

// TC-A02 按交付发放激励：批量 + 幂等 + 发券交易入账（不丢、不重）。
func TestA02_IssueIncentives(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")

	couponBody := map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 10, "batch_no": "GT-B-001",
	}
	e.issueCoupon(acc, couponBody)
	e.issueCoupon(acc, couponBody) // 重复提交同批次

	voucherBody := map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-V-001",
	}
	e.issueVoucher(acc, voucherBody)
	e.issueVoucher(acc, voucherBody) // 重复提交同批次

	coupons := e.coupons(acc)
	if len(coupons) != 10 {
		t.Fatalf("coupons = %d, want 10", len(coupons))
	}
	for _, c := range coupons {
		if c["status"] != "issued" {
			t.Errorf("coupon = %v", c)
		}
	}
	vouchers := e.vouchers(acc)
	if len(vouchers) != 1 {
		t.Errorf("vouchers = %d, want 1", len(vouchers))
	}

	// 两个批次各 1 条发券交易，重复提交不新增
	if got := e.countType(acc, "issue"); got != 2 {
		t.Errorf("issue txs = %d, want 2", got)
	}
}

// TC-A03 按学习扣费：付费记额度后，费用随学习进度逐次扣除（多次小额）。
func TestA03_LearningDeduction(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 20000, "GT-001") // 付费 20000 记入额度

	// 学第一节课扣 10000
	o1 := e.settle(map[string]any{
		"order_id": "O-GT-1", "customer_id": "stu-1", "account_id": acc,
		"product_id": "course-1", "scope": "course", "amount": 10000,
	})
	e.assertDetail(o1, []deduction{{Kind: "balance", Amount: 10000}})
	e.assertLedger(acc, 10000)

	// 学第二节课再扣 10000
	o2 := e.settle(map[string]any{
		"order_id": "O-GT-2", "customer_id": "stu-1", "account_id": acc,
		"product_id": "course-1", "scope": "course", "amount": 10000,
	})
	e.assertDetail(o2, []deduction{{Kind: "balance", Amount: 10000}})
	e.assertLedger(acc, 0)

	// 两笔消费交易 running balance 连续（10000 → 0），余额 = 交易求和
	// 接口按 id DESC 返回（最新在前），断言前翻转为时间正序
	txs := e.transactions(acc)
	var after []int64
	for _, tx := range txs {
		if tx["type"] == "consume" {
			after = append(after, centsOf(tx["balance_after"]))
		}
	}
	slices.Reverse(after)
	if len(after) != 2 || after[0] != 10000 || after[1] != 0 {
		t.Errorf("consume balance_after = %v, want [10000 0]", after)
	}
}

// TC-A04 满减券门槛（时机）：不达标不抵扣、达标抵扣并核销。
func TestA04_FullReductionThreshold(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 30000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "full_reduction", "threshold": 10000, "amount": 2000,
		"scope": "course", "expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})

	// 不达标：8000 < 10000
	o1 := e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 8000,
	})
	e.assertDetail(o1, []deduction{{Kind: "balance", Amount: 8000}})
	if got := e.coupons(acc)[0]["status"]; got != "issued" {
		t.Errorf("coupon status = %v, want issued", got)
	}

	// 达标：10000
	o2 := e.settle(map[string]any{
		"order_id": "O-GT-2", "account_id": acc, "scope": "course", "amount": 10000,
	})
	e.assertDetail(o2, []deduction{
		{Kind: "coupon", Amount: 2000},
		{Kind: "balance", Amount: 8000},
	})
	if got := e.coupons(acc)[0]["status"]; got != "used" {
		t.Errorf("coupon status = %v, want used", got)
	}
	e.assertLedger(acc, 30000-8000-8000)
}

// TC-A05 代金券优先于余额。
func TestA05_VoucherBeforeBalance(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 10000, "GT-001")
	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-V-001",
	})

	o := e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 10000,
	})
	e.assertDetail(o, []deduction{
		{Kind: "voucher", Amount: 2000},
		{Kind: "balance", Amount: 8000},
	})
	// 余额：10000 − 8000 = 2000（注：tests.md 中「余额 8000」为笔误）
	e.assertLedger(acc, 2000)
}

// TC-A06 混合结算顺序：券 → 代金券 → 余额；账本逐笔对得上。
func TestA06_MixedSettlement(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 10000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-V-001",
	})

	o := e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 10000,
	})
	// 9 折省 1000 → 代金券 2000 → 余额 7000
	e.assertDetail(o, []deduction{
		{Kind: "coupon", Amount: 1000},
		{Kind: "voucher", Amount: 2000},
		{Kind: "balance", Amount: 7000},
	})
	e.assertLedger(acc, 3000)
	if got := e.countType(acc, "redeem"); got != 2 {
		t.Errorf("redeem txs = %d, want 2", got)
	}
	if got := e.countType(acc, "consume"); got != 1 {
		t.Errorf("consume txs = %d, want 1", got)
	}
}

// TC-A07 余额不足整体回滚：无订单、券未核销、余额不变、无交易写入。
func TestA07_InsufficientBalanceRollback(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 5000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-V-001",
	})

	e.post("/orders", map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 10000,
	}).mustStatus(e, http.StatusUnprocessableEntity)

	// 订单不存在
	if got := e.get("/orders/O-GT-1").status; got != http.StatusNotFound {
		t.Errorf("order status = %d, want 404", got)
	}
	// 券均未核销
	for _, c := range e.coupons(acc) {
		if c["status"] != "issued" {
			t.Errorf("coupon = %v, want issued", c)
		}
	}
	for _, v := range e.vouchers(acc) {
		if v["status"] != "issued" {
			t.Errorf("voucher = %v, want issued", v)
		}
	}
	// 余额不变、无消费/核销交易（回滚干净）
	e.assertLedger(acc, 5000)
	if got := e.countType(acc, "consume") + e.countType(acc, "redeem"); got != 0 {
		t.Errorf("consume+redeem txs = %d, want 0", got)
	}
}

// TC-A08 过期课程券不可用：明细无券项，惰性流转为 expired。
func TestA08_ExpiredCoupon(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 10000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	e.backdateCoupon(acc, "GT-B-001") // 状态准备：过期

	o := e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 10000,
	})
	e.assertDetail(o, []deduction{{Kind: "balance", Amount: 10000}})
	if got := e.coupons(acc)[0]["status"]; got != "expired" {
		t.Errorf("coupon status = %v, want expired", got)
	}
}

// TC-A09 扣费幂等：重复提交返回同一订单，余额只扣一次、券只核销一次（不重）。
func TestA09_OrderIdempotent(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 20000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})

	body := map[string]any{
		"order_id": "O-GT-1", "account_id": acc, "scope": "course", "amount": 10000,
	}
	o1 := e.settle(body)
	o2 := e.settle(body) // 重复提交

	if o1["id"] != o2["id"] {
		t.Errorf("order ids differ: %v vs %v", o1["id"], o2["id"])
	}
	// 9 折省 1000，余额支付 9000；只扣一次 → 20000 − 9000 = 11000
	e.assertLedger(acc, 11000)
	if got := e.countType(acc, "consume"); got != 1 {
		t.Errorf("consume txs = %d, want 1", got)
	}
	if got := e.coupons(acc)[0]["status"]; got != "used" {
		t.Errorf("coupon status = %v, want used", got)
	}
}
