package billing_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	billinggorm "github.com/quanttide/qtcloud-pay/src/provider/internal/billing/gorm"
)

func newService(t *testing.T) *billing.Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&billing.BillingRule{}); err != nil {
		t.Fatal(err)
	}
	return billing.NewService(billinggorm.NewBillingRuleRepo())
}

func fullReduction(id int64, threshold, amount int64) billing.CouponInput {
	return billing.CouponInput{ID: id, Type: "full_reduction", Threshold: threshold, Amount: amount}
}

func discount(id int64, rate int) billing.CouponInput {
	return billing.CouponInput{ID: id, Type: "discount", Rate: rate}
}

func voucher(id int64, amount int64) billing.VoucherInput {
	return billing.VoucherInput{ID: id, Amount: amount}
}

func sum(plan []billing.Deduction) int64 {
	var total int64
	for _, d := range plan {
		total += d.Amount
	}
	return total
}

func TestCalculate(t *testing.T) {
	svc := newService(t)
	cases := []struct {
		name      string
		amount    int64
		coupons   []billing.CouponInput
		vouchers  []billing.VoucherInput
		balance   int64
		wantSum   int64 // 抵扣总额（等于订单金额时全额覆盖）
		wantKinds map[string]int
	}{
		{
			name: "无券全额余额", amount: 10000, balance: 100000,
			wantSum: 10000, wantKinds: map[string]int{"balance": 1},
		},
		{
			name: "仅满减", amount: 10000,
			coupons: []billing.CouponInput{fullReduction(1, 8000, 2000)}, balance: 100000,
			wantSum: 10000, wantKinds: map[string]int{"coupon": 1, "balance": 1},
		},
		{
			name: "满减门槛未满足", amount: 5000,
			coupons: []billing.CouponInput{fullReduction(1, 8000, 2000)}, balance: 100000,
			wantSum: 5000, wantKinds: map[string]int{"balance": 1},
		},
		{
			name: "满减取力度最大", amount: 10000, balance: 100000,
			coupons: []billing.CouponInput{
				fullReduction(1, 8000, 1000),
				fullReduction(2, 9000, 3000),
				fullReduction(3, 500, 50),
			},
			wantSum: 10000, wantKinds: map[string]int{"coupon": 1, "balance": 1},
		},
		{
			name: "仅折扣", amount: 10000,
			coupons: []billing.CouponInput{discount(1, 90)}, balance: 100000,
			wantSum: 10000, wantKinds: map[string]int{"coupon": 1, "balance": 1},
		},
		{
			name: "折扣向下取整", amount: 9999,
			coupons: []billing.CouponInput{discount(1, 90)}, balance: 100000,
			wantSum: 9999, wantKinds: map[string]int{"coupon": 1, "balance": 1},
		},
		{
			name: "满减加折扣", amount: 10000, balance: 100000,
			coupons: []billing.CouponInput{fullReduction(1, 8000, 2000), discount(2, 90)},
			// 满减 2000 → 剩余 8000 → 折扣 7200 → 余额 800
			wantSum: 10000, wantKinds: map[string]int{"coupon": 2, "balance": 1},
		},
		{
			name: "代金券全额抵扣", amount: 10000,
			vouchers: []billing.VoucherInput{voucher(1, 6000), voucher(2, 4000)}, balance: 100000,
			wantSum: 10000, wantKinds: map[string]int{"voucher": 2},
		},
		{
			name: "代金券部分抵扣", amount: 10000,
			vouchers: []billing.VoucherInput{voucher(1, 6000)}, balance: 100000,
			wantSum: 10000, wantKinds: map[string]int{"voucher": 1, "balance": 1},
		},
		{
			name: "混合抵扣", amount: 10000, balance: 100000,
			coupons:  []billing.CouponInput{fullReduction(1, 8000, 2000)},
			vouchers: []billing.VoucherInput{voucher(2, 3000)},
			// 满减 2000 → 代金券 3000 → 余额 5000
			wantSum: 10000, wantKinds: map[string]int{"coupon": 1, "voucher": 1, "balance": 1},
		},
		{
			name: "券额度超出订单", amount: 5000, balance: 100000,
			coupons:  []billing.CouponInput{fullReduction(1, 1000, 4000)},
			vouchers: []billing.VoucherInput{voucher(2, 3000)},
			// 满减 4000 → 剩余 1000 → 代金券 1000（部分使用）→ 余额 0
			wantSum: 5000, wantKinds: map[string]int{"coupon": 1, "voucher": 1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, err := svc.Calculate(c.amount, c.coupons, c.vouchers, c.balance)
			if err != nil {
				t.Fatalf("Calculate: %v", err)
			}
			if got := sum(plan); got != c.wantSum {
				t.Errorf("sum = %d, want %d (plan=%+v)", got, c.wantSum, plan)
			}
			kinds := map[string]int{}
			for _, d := range plan {
				kinds[d.Kind]++
			}
			for k, n := range c.wantKinds {
				if kinds[k] != n {
					t.Errorf("kind %s count = %d, want %d", k, kinds[k], n)
				}
			}
			// 抵扣总额不得超过订单金额
			if sum(plan) > c.amount {
				t.Errorf("sum %d exceeds amount %d", sum(plan), c.amount)
			}
		})
	}
}

func TestCalculate_Errors(t *testing.T) {
	svc := newService(t)

	// 金额非正
	if _, err := svc.Calculate(0, nil, nil, 100); !errors.Is(err, billing.ErrInvalidAmount) {
		t.Errorf("amount=0 err = %v, want ErrInvalidAmount", err)
	}
	if _, err := svc.Calculate(-1, nil, nil, 100); !errors.Is(err, billing.ErrInvalidAmount) {
		t.Errorf("amount<0 err = %v, want ErrInvalidAmount", err)
	}

	// 余额不足
	_, err := svc.Calculate(10000, nil, nil, 9999)
	if !errors.Is(err, billing.ErrInsufficientBalance) {
		t.Errorf("err = %v, want ErrInsufficientBalance", err)
	}
	// 券后仍不足
	_, err = svc.Calculate(10000, []billing.CouponInput{fullReduction(1, 1000, 2000)}, nil, 100)
	if !errors.Is(err, billing.ErrInsufficientBalance) {
		t.Errorf("err = %v, want ErrInsufficientBalance", err)
	}
}

func TestCalculate_InvalidCoupons(t *testing.T) {
	svc := newService(t)

	// 非法折扣券（rate 越界）被忽略
	plan, err := svc.Calculate(10000, []billing.CouponInput{
		{ID: 1, Type: "discount", Rate: 0},
		{ID: 2, Type: "discount", Rate: 101},
		{ID: 3, Type: "unknown"},
	}, nil, 10000)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if len(plan) != 1 || plan[0].Kind != billing.KindBalance {
		t.Errorf("plan = %+v, want only balance", plan)
	}

	// 折扣为 100%（全额抵扣）→ 生成抵扣项，无余额支付
	plan, err = svc.Calculate(100, []billing.CouponInput{{ID: 1, Type: "discount", Rate: 100}}, nil, 100)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if len(plan) != 1 || plan[0].Kind != billing.KindCoupon || plan[0].Amount != 100 {
		t.Errorf("plan = %+v, want coupon 100", plan)
	}
}

func TestRules(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&billing.BillingRule{}); err != nil {
		t.Fatal(err)
	}
	svc := billing.NewService(billinggorm.NewBillingRuleRepo())

	// 空表
	rules, err := svc.Rules(context.Background(), db)
	if err != nil || len(rules) != 0 {
		t.Fatalf("Rules(empty) = %d, %v", len(rules), err)
	}

	// 按优先级排序
	db.Create(&billing.BillingRule{Priority: 2, Kind: "voucher"})
	db.Create(&billing.BillingRule{Priority: 1, Kind: "coupon"})
	rules, err = svc.Rules(context.Background(), db)
	if err != nil || len(rules) != 2 {
		t.Fatalf("Rules = %d, %v", len(rules), err)
	}
	if rules[0].Kind != "coupon" || rules[1].Kind != "voucher" {
		t.Errorf("rules order = %+v", rules)
	}
}

func TestRules_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := billing.NewService(billinggorm.NewBillingRuleRepo())
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if _, err := svc.Rules(context.Background(), db); err == nil {
		t.Fatal("expected error on closed db")
	}
}
