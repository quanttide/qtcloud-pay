package itest

import (
	"testing"
)

// TC-C01 开户与预存充值。
func TestC01_RechargePrepaid(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("cloud-1")

	e.recharge(acc, 10000, "Y-001")

	e.assertLedger(acc, 10000)
	if got := e.countType(acc, "recharge"); got != 1 {
		t.Errorf("recharge txs = %d, want 1", got)
	}
}

// TC-C02 发放全场代金券。
func TestC02_IssueAllScopeVoucher(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("cloud-1")

	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "Y-V-001",
	})
	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "Y-V-001",
	}) // 重复

	if got := e.vouchers(acc)[0]["status"]; got != "issued" {
		t.Errorf("voucher status = %v", got)
	}
	if got := e.countType(acc, "issue"); got != 1 {
		t.Errorf("issue txs = %d, want 1", got)
	}
}

// TC-C03 按量多次消费：余额连续扣减；代金券全额抵现时余额不扣。
func TestC03_MeteredConsumption(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("cloud-1")
	e.recharge(acc, 10000, "Y-001")

	// 第一次：3000，余额 7000（此时尚无代金券，纯余额）
	e.settle(map[string]any{
		"order_id": "O-Y-1", "account_id": acc, "scope": "cloud", "amount": 3000,
	})
	e.assertLedger(acc, 7000)

	// 第二次前发放代金券：2000 由代金券全额抵现，余额不扣
	e.issueVoucher(acc, map[string]any{
		"amount": 2000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "Y-V-001",
	})
	o2 := e.settle(map[string]any{
		"order_id": "O-Y-2", "account_id": acc, "scope": "cloud", "amount": 2000,
	})
	e.assertDetail(o2, []deduction{{Kind: "voucher", Amount: 2000}})
	e.assertLedger(acc, 7000)

	// 第三次：5000，余额 2000
	e.settle(map[string]any{
		"order_id": "O-Y-3", "account_id": acc, "scope": "cloud", "amount": 5000,
	})
	e.assertLedger(acc, 2000)

	// 账单运行余额连续正确：充值 10000 → 消费 3000 → 发券（不变）→ 核销（不变）→ 消费 5000
	stmt := e.statement(acc)
	entries := stmt["entries"].([]any)
	if len(entries) != 5 { // 充值 + 消费 + 发券 + 核销 + 消费
		t.Fatalf("entries = %d, want 5", len(entries))
	}
	want := []int64{10000, 7000, 7000, 7000, 2000}
	for i, w := range want {
		if int64(entries[i].(map[string]any)["running_balance"].(float64)) != w {
			t.Errorf("running_balance[%d] = %v, want %d", i, entries[i], w)
		}
	}
	// 代金券抵现应有一条核销交易
	if got := e.countType(acc, "redeem"); got != 1 {
		t.Errorf("redeem txs = %d, want 1", got)
	}
}
