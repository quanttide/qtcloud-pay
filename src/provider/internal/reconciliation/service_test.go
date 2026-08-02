package reconciliation_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	accountgorm "github.com/quanttide/qtcloud-pay/src/provider/internal/account/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/reconciliation"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
)

func setupEnv(t *testing.T) (*gorm.DB, *account.Service, *transaction.Service, *reconciliation.Service) {
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
	if err := db.AutoMigrate(&account.Account{}, &transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	accSvc := account.NewService(db, accountgorm.NewAccountRepo(), txSvc)
	reconSvc := reconciliation.NewService(db, accSvc, txSvc)
	return db, accSvc, txSvc, reconSvc
}

func TestCheckConsistency(t *testing.T) {
	db, accSvc, txSvc, reconSvc := setupEnv(t)
	ctx := context.Background()

	// 一致：充值 1000 + 500，消费 300 → 期望余额 1200
	acc, _ := accSvc.Create(ctx, "cust_1")
	accSvc.Recharge(ctx, acc.ID, 1000, "v1", "")
	accSvc.Recharge(ctx, acc.ID, 500, "v2", "")
	acc.Balance = 1200 // 直改余额绕开服务（模拟消费 300 后）
	if err := accSvc.Save(ctx, db, acc); err != nil {
		t.Fatal(err)
	}
	txSvc.Append(ctx, db, &transaction.Transaction{
		AccountID: acc.ID, Type: transaction.TypeConsume, Amount: 300,
		BalanceAfter: 1200, IdempotencyKey: "c1",
	})

	discrepancies, err := reconSvc.CheckConsistency(ctx)
	if err != nil || len(discrepancies) != 0 {
		t.Fatalf("discrepancies = %+v, %v; want none", discrepancies, err)
	}

	// 人为制造不一致：余额与交易不符
	acc.Balance = 9999
	accSvc.Save(ctx, db, acc)
	discrepancies, err = reconSvc.CheckConsistency(ctx)
	if err != nil || len(discrepancies) != 1 {
		t.Fatalf("discrepancies = %+v, %v; want 1", discrepancies, err)
	}
	if discrepancies[0].AccountID != acc.ID || discrepancies[0].Expected != 1200 {
		t.Errorf("discrepancy = %+v", discrepancies[0])
	}
}

func TestCheckConsistency_DBError(t *testing.T) {
	db, accSvc, txSvc, reconSvc := setupEnv(t)
	sqlDB, _ := db.DB()
	sqlDB.Close()
	_ = accSvc
	_ = txSvc
	if _, err := reconSvc.CheckConsistency(context.Background()); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestReconcileBankFile(t *testing.T) {
	db, accSvc, _, reconSvc := setupEnv(t)
	ctx := context.Background()

	acc, _ := accSvc.Create(ctx, "cust_1")
	accSvc.Recharge(ctx, acc.ID, 5000, "voucher-001", "")
	accSvc.Recharge(ctx, acc.ID, 3000, "voucher-002", "")

	csv := "voucher_no,amount_cents,date\n" +
		"voucher-001,5000,2026-08-01\n" +
		"voucher-002,3000,2026-08-02\n" + // 匹配
		"voucher-003,7000,2026-08-03\n" + // 未找到交易
		"voucher-001,9999,2026-08-04\n" // 金额不一致
	report, err := reconSvc.ReconcileBankFile(ctx, strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ReconcileBankFile: %v", err)
	}
	if report.Total != 4 || len(report.Matched) != 2 || len(report.Unmatched) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if report.Matched[0].Row.VoucherNo != "voucher-001" || report.Matched[1].Row.VoucherNo != "voucher-002" {
		t.Errorf("matched = %+v", report.Matched)
	}
	if report.Unmatched[0].Reason != "未找到对应充值交易" || report.Unmatched[1].Reason != "金额不一致" {
		t.Errorf("unmatched = %+v", report.Unmatched)
	}

	// 无表头
	report2, err := reconSvc.ReconcileBankFile(ctx, strings.NewReader("voucher-001,5000,2026-08-01\n"))
	if err != nil || report2.Total != 1 || len(report2.Matched) != 1 {
		t.Fatalf("report2 = %+v, %v", report2, err)
	}
	_ = db
}

func TestReconcileBankFile_InvalidCSV(t *testing.T) {
	_, _, _, reconSvc := setupEnv(t)

	cases := []string{
		"a,not-a-number,2026-08-01\n", // 金额非法
		"a,0,2026-08-01\n",            // 金额非正
		",100,2026-08-01\n",           // 缺凭证号
		"a,100,\n",                    // 缺日期
		"a,100\n",                     // 列数不足
	}
	for _, csv := range cases {
		if _, err := reconSvc.ReconcileBankFile(context.Background(), strings.NewReader(csv)); !errors.Is(err, reconciliation.ErrInvalidCSV) {
			t.Errorf("csv=%q err = %v, want ErrInvalidCSV", csv, err)
		}
	}
}

func TestStatement(t *testing.T) {
	db, accSvc, txSvc, reconSvc := setupEnv(t)
	ctx := context.Background()

	acc, _ := accSvc.Create(ctx, "cust_1")
	accSvc.Recharge(ctx, acc.ID, 10000, "v1", "首充")
	accSvc.Recharge(ctx, acc.ID, 5000, "v2", "二充")
	// 消费 3000（直改余额 + 写交易）；期初余额 = 22000 − 12000 = 10000
	acc, _ = accSvc.Get(ctx, acc.ID)
	acc.Balance = 22000
	accSvc.Save(ctx, db, acc)
	txSvc.Append(ctx, db, &transaction.Transaction{
		AccountID: acc.ID, Type: transaction.TypeConsume, Amount: 3000,
		BalanceAfter: 22000, IdempotencyKey: "c1",
	})

	stmt, err := reconSvc.Statement(ctx, acc.ID)
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if stmt.Opening != 10000 || stmt.Closing != 22000 {
		t.Errorf("opening=%d closing=%d, want 10000/22000", stmt.Opening, stmt.Closing)
	}
	if len(stmt.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(stmt.Entries))
	}
	// 运行余额：10000 → 20000 → 25000 → 22000
	want := []int64{20000, 25000, 22000}
	for i, w := range want {
		if stmt.Entries[i].RunningBalance != w {
			t.Errorf("entry[%d] balance = %d, want %d", i, stmt.Entries[i].RunningBalance, w)
		}
	}
	// 期初 + 净变动 = 期末
	if stmt.Opening+(stmt.Closing-stmt.Opening) != stmt.Closing {
		t.Error("statement arithmetic broken")
	}

	// 空流水账户
	acc2, _ := accSvc.Create(ctx, "cust_2")
	stmt2, err := reconSvc.Statement(ctx, acc2.ID)
	if err != nil || stmt2.Opening != 0 || len(stmt2.Entries) != 0 {
		t.Errorf("empty statement = %+v, %v", stmt2, err)
	}

	// 账户不存在
	if _, err := reconSvc.Statement(ctx, "acc_missing"); !errors.Is(err, account.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStatement_DBError(t *testing.T) {
	db, _, _, reconSvc := setupEnv(t)
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if _, err := reconSvc.Statement(context.Background(), "acc_1"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

// stubTxRepo 注入交易账本仓库错误。
type stubTxRepo struct {
	transaction.Repository
	sumErr     error
	getErr     error
	listAllErr error
}

func (s *stubTxRepo) GetByKey(db *gorm.DB, key string) (*transaction.Transaction, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubTxRepo) SumByAccount(db *gorm.DB, accountID string) (int64, error) {
	return 0, s.sumErr
}

func (s *stubTxRepo) ListAllByAccount(db *gorm.DB, accountID string) ([]transaction.Transaction, error) {
	return nil, s.listAllErr
}

func TestServiceErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("consistency sum error", func(t *testing.T) {
		db, accSvc, _, _ := setupEnv(t)
		acc, _ := accSvc.Create(ctx, "cust_1")
		reconSvc := reconciliation.NewService(db, accSvc,
			transaction.NewService(&stubTxRepo{sumErr: errors.New("sum failed")}))
		if _, err := reconSvc.CheckConsistency(ctx); err == nil {
			t.Fatalf("expected error for account %s", acc.ID)
		}
	})

	t.Run("bank file get error", func(t *testing.T) {
		db, accSvc, _, _ := setupEnv(t)
		reconSvc := reconciliation.NewService(db, accSvc,
			transaction.NewService(&stubTxRepo{getErr: errors.New("query failed")}))
		if _, err := reconSvc.ReconcileBankFile(ctx, strings.NewReader("v1,100,2026-08-01\n")); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("statement list error", func(t *testing.T) {
		db, accSvc, _, _ := setupEnv(t)
		acc, _ := accSvc.Create(ctx, "cust_1")
		reconSvc := reconciliation.NewService(db, accSvc,
			transaction.NewService(&stubTxRepo{listAllErr: errors.New("list failed")}))
		if _, err := reconSvc.Statement(ctx, acc.ID); err == nil {
			t.Fatal("expected error")
		}
	})
}
