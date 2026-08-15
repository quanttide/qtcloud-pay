package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/idempotency"
)

var (
	// ErrExists 账户已存在。
	ErrExists = errors.New("account: already exists")
	// ErrNotFound 账户不存在。
	ErrNotFound = errors.New("account: not found")
	// ErrInvalidAmount 金额必须为正整数（分）。
	ErrInvalidAmount = errors.New("account: invalid amount")
	// ErrInvalidRecharge 缺少打款凭证号。
	ErrInvalidRecharge = errors.New("account: voucher no required")
	// ErrInvalidRefund 缺少退款凭证号。
	ErrInvalidRefund = errors.New("account: refund voucher no required")
	// ErrInsufficientBalance 余额不足，无法退款。
	ErrInsufficientBalance = errors.New("account: insufficient balance")
)

// Service 账户与余额服务。
type Service struct {
	db    *gorm.DB
	repo  Repository
	txSvc *transaction.Service
}

// NewService 创建账户服务。
func NewService(db *gorm.DB, repo Repository, txSvc *transaction.Service) *Service {
	return &Service{db: db, repo: repo, txSvc: txSvc}
}

// Create 创建账户；ID 由服务端生成（acc_ 前缀）。
func (s *Service) Create(ctx context.Context, customerID string) (*Account, error) {
	if customerID == "" {
		return nil, errors.New("account: customer id required")
	}
	a := &Account{ID: newAccountID(), CustomerID: customerID}
	if err := s.repo.Create(s.db, a); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrExists
		}
		return nil, err
	}
	return a, nil
}

// Get 查询账户与余额。
func (s *Service) Get(ctx context.Context, id string) (*Account, error) {
	a, err := s.repo.Get(s.db, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return a, err
}

// List 查询全部账户（供对账）。
func (s *Service) List(ctx context.Context) ([]Account, error) {
	return s.repo.List(s.db)
}

// GetByCustomer 按客户标识查询账户；不存在返回 ErrNotFound。
func (s *Service) GetByCustomer(ctx context.Context, customerID string) (*Account, error) {
	a, err := s.repo.GetByCustomer(s.db, customerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return a, err
}

// Delete 删除账户及全部关联数据（流水/订单/券）。运维用：清空测试数据。
// 事务内先删子表后删账户；账户不存在返回 ErrNotFound。
// 表名为内部常量（避免跨模块 import 环），不带用户输入。
func (s *Service) Delete(ctx context.Context, id string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.repo.Get(tx, id); errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		for _, table := range []string{"transactions", "orders", "vouchers", "coupons"} {
			if err := tx.Exec("DELETE FROM "+table+" WHERE account_id = ?", id).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&Account{}, "id = ?", id).Error
	})
	return err
}

// ListTransactions 查询账户流水（委托交易账本模块）。
func (s *Service) ListTransactions(ctx context.Context, accountID string, limit, offset int) ([]transaction.Transaction, error) {
	return s.txSvc.List(ctx, s.db, accountID, limit, offset)
}

// Lock 锁定账户行并返回（供结算编排：同账户结算串行化）。
func (s *Service) Lock(ctx context.Context, db *gorm.DB, id string) (*Account, error) {
	a, err := s.repo.GetForUpdate(db, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return a, err
}

// Save 写回账户（供结算编排：扣余额后保存）。
func (s *Service) Save(ctx context.Context, db *gorm.DB, a *Account) error {
	a.UpdatedAt = time.Now()
	return s.repo.Update(db, a)
}

// Recharge 充值登记（对公打款入账）。
// 幂等键为打款凭证号；重复提交同凭证号不会重复入账。
func (s *Service) Recharge(ctx context.Context, accountID string, amount int64, voucherNo, note string) error {
	if accountID == "" {
		return errors.New("account: account id required")
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if voucherNo == "" {
		return ErrInvalidRecharge
	}
	key, err := idempotency.Key(idempotency.Recharge, voucherNo)
	if err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		exists, err := s.txSvc.Exists(ctx, tx, key)
		if err != nil {
			return err
		}
		if exists {
			return nil // 幂等：已入账，不重复处理
		}
		acc, err := s.Lock(ctx, tx, accountID)
		if err != nil {
			return err
		}
		acc.Balance += amount
		if err := s.Save(ctx, tx, acc); err != nil {
			return err
		}
		return s.txSvc.Append(ctx, tx, &transaction.Transaction{
			AccountID:      accountID,
			Type:           transaction.TypeRecharge,
			Amount:         amount,
			BalanceAfter:   acc.Balance,
			IdempotencyKey: key,
			Note:           note,
		})
	})
	if errors.Is(err, transaction.ErrDuplicateKey) {
		return nil // 并发下另一请求已入账，本请求整体回滚，视为成功
	}
	return err
}

// Refund 退款登记（多退：对公退款出账，余额扣减）。
// 与 Recharge 对称：幂等键为退款凭证号；重复提交同凭证号不会重复退款；
// 余额不足整体回滚并返回 ErrInsufficientBalance。
func (s *Service) Refund(ctx context.Context, accountID string, amount int64, voucherNo, note string) error {
	if accountID == "" {
		return errors.New("account: account id required")
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if voucherNo == "" {
		return ErrInvalidRefund
	}
	key, err := idempotency.Key(idempotency.Refund, voucherNo)
	if err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		exists, err := s.txSvc.Exists(ctx, tx, key)
		if err != nil {
			return err
		}
		if exists {
			return nil // 幂等：已退款，不重复处理
		}
		acc, err := s.Lock(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if acc.Balance < amount {
			return ErrInsufficientBalance
		}
		acc.Balance -= amount
		if err := s.Save(ctx, tx, acc); err != nil {
			return err
		}
		return s.txSvc.Append(ctx, tx, &transaction.Transaction{
			AccountID:      accountID,
			Type:           transaction.TypeRefund,
			Amount:         amount,
			BalanceAfter:   acc.Balance,
			IdempotencyKey: key,
			Note:           note,
		})
	})
	if errors.Is(err, transaction.ErrDuplicateKey) {
		return nil // 并发下另一请求已退款，本请求整体回滚，视为成功
	}
	return err
}

// newAccountID 生成账户业务号（acc_ + 16 位十六进制）。
func newAccountID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败属于环境级错误
	}
	return "acc_" + hex.EncodeToString(b)
}
