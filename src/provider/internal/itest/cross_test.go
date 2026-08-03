package itest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

// TC-X01 已使用券不可复用：明细无券项，余额补足，券不重复核销。
func TestX01_UsedCouponNotReusable(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 20000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})

	o1 := e.settle(map[string]any{
		"order_id": "O-1", "account_id": acc, "scope": "course", "amount": 10000,
	})
	e.assertDetail(o1, []deduction{{Kind: "coupon", Amount: 1000}, {Kind: "balance", Amount: 9000}})

	// 第二次结算：券已 used，不再参与
	o2 := e.settle(map[string]any{
		"order_id": "O-2", "account_id": acc, "scope": "course", "amount": 10000,
	})
	e.assertDetail(o2, []deduction{{Kind: "balance", Amount: 10000}})
	e.assertLedger(acc, 1000)
	if got := e.countType(acc, "redeem"); got != 1 {
		t.Errorf("redeem txs = %d, want 1（券只核销一次）", got)
	}
}

// TC-X02 指定商品券仅限该商品。
func TestX02_ProductScopeCoupon(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 20000, "GT-001")
	e.issueCoupon(acc, map[string]any{
		"type": "full_reduction", "threshold": 10000, "amount": 5000,
		"scope": "product", "product_id": "course-1",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})

	// 其他商品：不参与
	o1 := e.settle(map[string]any{
		"order_id": "O-1", "account_id": acc, "product_id": "course-2",
		"scope": "course", "amount": 10000,
	})
	e.assertDetail(o1, []deduction{{Kind: "balance", Amount: 10000}})
	if got := e.coupons(acc)[0]["status"]; got != "issued" {
		t.Errorf("coupon status = %v, want issued", got)
	}

	// 指定商品：正常核销
	o2 := e.settle(map[string]any{
		"order_id": "O-2", "account_id": acc, "product_id": "course-1",
		"scope": "course", "amount": 10000,
	})
	e.assertDetail(o2, []deduction{{Kind: "coupon", Amount: 5000}, {Kind: "balance", Amount: 5000}})
}

// TC-X03 过期状态查询可见。
func TestX03_ExpiredStatusVisible(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	e.issueCoupon(acc, map[string]any{
		"type": "discount", "rate": 95, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-002",
	})
	e.backdateCoupon(acc, "GT-B-001") // 第一张过期

	statuses := map[string]int{}
	for _, c := range e.coupons(acc) {
		statuses[c["status"].(string)]++
	}
	if statuses["expired"] != 1 || statuses["issued"] != 1 {
		t.Errorf("statuses = %v, want expired=1 issued=1", statuses)
	}
}

// TC-X04 并发同订单号结算：仅一笔生效（不重）。
func TestX04_ConcurrentSameOrder(t *testing.T) {
	e := newEnv(t)
	acc := e.createAccount("stu-1")
	e.recharge(acc, 20000, "GT-001")

	body, _ := json.Marshal(map[string]any{
		"order_id": "O-C-1", "account_id": acc, "scope": "course", "amount": toAmount(10000),
	})
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := e.client.Post(e.base+"/orders", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Error(err)
				return
			}
			statuses <- r.StatusCode
			r.Body.Close()
		}()
	}
	wg.Wait()
	close(statuses)
	for s := range statuses {
		if s != http.StatusCreated {
			t.Errorf("status = %d, want 201", s)
		}
	}

	// 仅一笔生效：余额只扣一次、一条消费交易、订单唯一
	e.assertLedger(acc, 10000)
	if got := e.countType(acc, "consume"); got != 1 {
		t.Errorf("consume txs = %d, want 1", got)
	}
	o := e.order("O-C-1")
	if centsOf(o["amount"]) != 10000 {
		t.Errorf("order = %v", o)
	}
}

// TC-X05 三业务总对账：混合流水后逐账户核对、交易可追溯、账单导出。
func TestX05_GlobalReconciliation(t *testing.T) {
	e := newEnv(t)

	// 课堂：充值 20000 + 折扣券消费 10000（券 1000 / 余额 9000）
	stu := e.createAccount("stu-1")
	e.recharge(stu, 20000, "GT-001")
	e.issueCoupon(stu, map[string]any{
		"type": "discount", "rate": 90, "scope": "course",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	o1 := e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": stu, "scope": "course", "amount": 10000,
	})

	// 数据：充值 800000 + 满减券消费 800000
	teacher := e.createAccount("teacher-1")
	e.recharge(teacher, 800000, "SJ-001")
	e.issueCoupon(teacher, map[string]any{
		"type": "full_reduction", "threshold": 500000, "amount": 100000,
		"scope": "data", "expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-B-001",
	})
	o2 := e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": teacher, "scope": "data", "amount": 800000,
	})

	// 云：充值 10000 + 按量消费 3000
	cloud := e.createAccount("cloud-1")
	e.recharge(cloud, 10000, "Y-001")
	o3 := e.settle(map[string]any{
		"order_id": "O-Y-1", "account_id": cloud, "scope": "cloud", "amount": 3000,
	})

	// 逐账户：余额 = 交易求和，无一致性差异
	e.assertLedger(stu, 11000)      // 20000 − (10000−1000)
	e.assertLedger(teacher, 100000) // 800000 − (800000−100000)
	e.assertLedger(cloud, 7000)     // 10000 − 3000

	// 交易可追溯：消费带 order_id；券核销关联订单；订单结算明细完整
	for _, o := range []map[string]any{o1, o2, o3} {
		if len(e.detail(o)) == 0 {
			t.Errorf("order %v settle_detail empty", o["id"])
		}
	}
	for _, acc := range []string{stu, teacher, cloud} {
		for _, tx := range e.transactions(acc) {
			if tx["type"] == "consume" && tx["order_id"] == "" {
				t.Errorf("consume without order_id: %v", tx)
			}
		}
	}
	if got := e.coupons(stu)[0]["order_id"]; got != "O-GT-1" {
		t.Errorf("coupon order_id = %v, want O-GT-1", got)
	}

	// 账单导出：期初 + 净变动 = 期末
	for _, acc := range []string{stu, teacher, cloud} {
		stmt := e.statement(acc)
		opening := centsOf(stmt["opening_balance"])
		closing := centsOf(stmt["closing_balance"])
		if opening+e.netFlow(acc) != closing {
			t.Errorf("account %s: opening %d + net %d != closing %d", acc, opening, e.netFlow(acc), closing)
		}
	}
}

// TC-X06 三业务总闭环：模拟账户先行，全程不依赖支付通道。
func TestX06_FullClosedLoop(t *testing.T) {
	e := newEnv(t)

	// 未挂载支付渠道：/pay 不存在（反直觉点：不接入支付完成闭环）
	if got := e.get("/pay").status; got != http.StatusNotFound {
		t.Errorf("/pay status = %d, want 404", got)
	}

	// 课堂旅程：打款 → 记额度 → 发券 → 学习扣费
	stu := e.createAccount("stu-1")
	e.recharge(stu, 20000, "GT-001")
	e.issueCoupon(stu, map[string]any{
		"type": "full_reduction", "threshold": 10000, "amount": 2000,
		"scope": "course", "expires_at": futureExpiry(), "count": 1, "batch_no": "GT-B-001",
	})
	e.settle(map[string]any{
		"order_id": "O-GT-1", "account_id": stu, "scope": "course", "amount": 10000,
	})

	// 数据旅程
	teacher := e.createAccount("teacher-1")
	e.recharge(teacher, 800000, "SJ-001")
	e.issueVoucher(teacher, map[string]any{
		"amount": 50000, "scope": "all",
		"expires_at": futureExpiry(), "count": 1, "batch_no": "SJ-V-001",
	})
	e.settle(map[string]any{
		"order_id": "O-SJ-1", "account_id": teacher, "scope": "data", "amount": 800000,
	})

	// 云旅程
	cloud := e.createAccount("cloud-1")
	e.recharge(cloud, 10000, "Y-001")
	e.settle(map[string]any{
		"order_id": "O-Y-1", "account_id": cloud, "scope": "cloud", "amount": 3000,
	})

	// 总验收：三个账户账本逐笔对得上
	e.assertLedger(stu, 12000)     // 20000 − (10000−2000)
	e.assertLedger(teacher, 50000) // 800000 − (800000−50000)
	e.assertLedger(cloud, 7000)    // 10000 − 3000
}
