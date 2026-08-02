package itest

import (
	"net/http"
	"testing"
)

// TC-B01 开户与付费记额度：预收对公打款入账，余额 = 交易求和；重复提交不重复入账。
func TestB01_RechargeLarge(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")

	e.recharge(acc, 800000, "SJ-001")
	e.recharge(acc, 800000, "SJ-001") // 重复

	e.assertLedger(acc, 800000)
	if got := e.countType(acc, "recharge"); got != 1 {
		t.Errorf("recharge txs = %d, want 1", got)
	}
}

// TC-B02 发放数据服务激励券：满减券 + 代金券，批次幂等。
func TestB02_IssueDataIncentives(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")

	couponBody := map[string]any{
		"type": "full_reduction", "threshold": 500000, "amount": 100000,
		"scope": "data", "expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	}
	e.issueCoupon(acc, couponBody)
	e.issueCoupon(acc, couponBody) // 重复

	voucherBody := map[string]any{
		"amount": 50000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-V-001",
	}
	e.issueVoucher(acc, voucherBody)
	e.issueVoucher(acc, voucherBody) // 重复

	if got := e.coupons(acc)[0]["status"]; got != "issued" {
		t.Errorf("coupon status = %v", got)
	}
	if got := e.vouchers(acc)[0]["status"]; got != "issued" {
		t.Errorf("voucher status = %v", got)
	}
	if got := e.countType(acc, "issue"); got != 2 {
		t.Errorf("issue txs = %d, want 2", got)
	}
}

// TC-B03 按交付进度扣费：分期结算 + 数据量弹性计费，券 → 代金券 → 余额逐笔对得上。
func TestB03_ProgressBasedBilling(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 800000, "SJ-001")
	e.issueCoupon(acc, map[string]any{
		"type": "full_reduction", "threshold": 500000, "amount": 100000,
		"scope": "data", "expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	})
	e.issueVoucher(acc, map[string]any{
		"amount": 50000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-V-001",
	})

	// 第一期：按交付进度 500000（满减 100000 + 代金券 50000 + 余额 350000）
	o1 := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 500000,
	})
	e.assertDetail(o1, []deduction{
		{Kind: "coupon", Amount: 100000},
		{Kind: "voucher", Amount: 50000},
		{Kind: "balance", Amount: 350000},
	})
	e.assertLedger(acc, 450000)

	// 第二期：按实际数据量 300000（纯余额，弹性计费）
	o2 := e.settle(map[string]any{
		"order_id": "O-SJ-2", "account_id": acc, "scope": "data", "amount": 300000,
	})
	e.assertDetail(o2, []deduction{{Kind: "balance", Amount: 300000}})
	e.assertLedger(acc, 150000)

	// 账本：2 条消费 + 1 条核销 + 1 条抵现，逐笔对得上
	if got := e.countType(acc, "consume"); got != 2 {
		t.Errorf("consume txs = %d, want 2", got)
	}
	if got := e.countType(acc, "redeem"); got != 2 {
		t.Errorf("redeem txs = %d, want 2", got)
	}
}

// TC-B04 多退少补：按实际用量结算后，多退（退款登记，幂等）少补（再充值）。
func TestB04_SettleSettleRefundRecharge(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 800000, "SJ-001") // 按预估合同额预收

	// 实际用量 700000 → 多退 100000 回原路
	e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 700000,
	})
	e.assertLedger(acc, 100000)

	e.refund(acc, 100000, "SJ-R-001")
	e.refund(acc, 100000, "SJ-R-001") // 重复：同凭证号不重复退
	e.assertLedger(acc, 0)
	if got := e.countType(acc, "refund"); got != 1 {
		t.Errorf("refund txs = %d, want 1", got)
	}

	// 少补：实际用量超出预存 → 余额不足整体回滚 → 补款后再结算
	e.post("/orders", map[string]any{
		"order_id": "O-SJ-2", "account_id": acc, "scope": "data", "amount": 300000,
	}).mustStatus(e, http.StatusUnprocessableEntity)
	e.assertLedger(acc, 0) // 回滚干净

	e.recharge(acc, 300000, "SJ-002") // 少补
	e.settle(map[string]any{
		"order_id": "O-SJ-2", "account_id": acc, "scope": "data", "amount": 300000,
	})
	e.assertLedger(acc, 0)
}

// TC-B05 多满减券选力度最大（减额最大）。
func TestB05_PickBestFullReduction(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 800000, "SJ-001")
	e.issueCoupon(acc, map[string]any{
		"type": "full_reduction", "threshold": 500000, "amount": 100000,
		"scope": "data", "expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	})
	e.issueCoupon(acc, map[string]any{
		"type": "full_reduction", "threshold": 800000, "amount": 200000,
		"scope": "data", "expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-002",
	})

	o := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 800000,
	})
	// 核销满 800000 减 200000 的券（力度最大）
	e.assertDetail(o, []deduction{
		{Kind: "coupon", Amount: 200000},
		{Kind: "balance", Amount: 600000},
	})
	// 未选中的券保持 issued
	coupons := e.coupons(acc)
	var issued, used int
	for _, c := range coupons {
		switch c["status"] {
		case "used":
			used++
		case "issued":
			issued++
		}
	}
	if used != 1 || issued != 1 {
		t.Errorf("coupons used=%d issued=%d, want 1/1", used, issued)
	}
}

// TC-B06 课程券不能用于数据订单（跨业务隔离）。
func TestB06_ScopeIsolation(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 200000, "SJ-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	})

	o := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 100000,
	})
	// 课程券不参与数据订单
	e.assertDetail(o, []deduction{{Kind: "balance", Amount: 100000}})
	if got := e.coupons(acc)[0]["status"]; got != "issued" {
		t.Errorf("coupon status = %v, want issued", got)
	}
}

// TC-B07 多折扣券选力度最大（rate 最低，省得最多）。
func TestB07_PickBestDiscount(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 100000, "SJ-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "data",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	})
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 80, "scope": "data",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-002",
	})

	o := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 100000,
	})
	// 8 折：省 20000，力度最大
	e.assertDetail(o, []deduction{
		{Kind: "coupon", Amount: 20000},
		{Kind: "balance", Amount: 80000},
	})
	coupons := e.coupons(acc)
	var used, issued int
	for _, c := range coupons {
		switch c["status"] {
		case "used":
			used++
		case "issued":
			issued++
		}
	}
	if used != 1 || issued != 1 {
		t.Errorf("coupons used=%d issued=%d, want 1/1", used, issued)
	}
}

// TC-B08 代金券面值大于剩余应付：只抵应付，不找零（力度约定）。
func TestB08_VoucherNoChange(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("teacher-1")
	e.recharge(acc, 30000, "SJ-001")
	e.issueVoucher(acc, map[string]any{
		"amount": 50000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-V-001",
	})

	o := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": acc, "scope": "data", "amount": 30000,
	})
	e.assertDetail(o, []deduction{{Kind: "voucher", Amount: 30000}})
	// 余额分文未动
	e.assertLedger(acc, 30000)
	if got := e.vouchers(acc)[0]["status"]; got != "used" {
		t.Errorf("voucher status = %v, want used", got)
	}
}
