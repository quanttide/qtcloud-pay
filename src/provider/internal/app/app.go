// Package app 服务端组装：数据库打开、依赖注入与路由注册。
// cmd/server 与集成测试（internal/itest）共用，保证装配一致。
package app

import (
	"fmt"
	"net/http"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	accountgorm "github.com/quanttide/qtcloud-pay/src/provider/internal/account/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	billinggorm "github.com/quanttide/qtcloud-pay/src/provider/internal/billing/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/alipay"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/wechat"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	coupongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/coupon/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	ordergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/order/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/reconciliation"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
)

// Open 打开数据库并迁移全部模型（driver: sqlite/postgres，dsn 对应驱动格式）。
func Open(driver, dsn string) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch driver {
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	default: // sqlite（开发默认）
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	}
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&account.Account{}, &transaction.Transaction{},
		&coupon.Coupon{}, &voucher.Voucher{},
		&order.Order{}, &billing.BillingRule{},
	); err != nil {
		return nil, err
	}
	if driver != "postgres" {
		// SQLite 单写者：限制单连接，避免并发写事务触发 database is locked
		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}
		sqlDB.SetMaxOpenConns(1)
	}
	return db, nil
}

// OpenDB 按环境变量（DB_DRIVER / DATABASE_URL / DB_SQLITE_DSN）打开数据库。
func OpenDB() (*gorm.DB, error) {
	if os.Getenv("DB_DRIVER") == "postgres" {
		return Open("postgres", os.Getenv("DATABASE_URL"))
	}
	dsn := os.Getenv("DB_SQLITE_DSN")
	if dsn == "" {
		dsn = "qtcloud-pay.db"
	}
	return Open("sqlite", dsn)
}

// BuildMux 组装全部模块并返回路由；channelName 非空时挂载支付渠道。
func BuildMux(db *gorm.DB, channelName string) (*http.ServeMux, error) {
	// 账本核心
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	accSvc := account.NewService(db, accountgorm.NewAccountRepo(), txSvc)
	couponSvc := coupon.NewService(db, coupongorm.NewCouponRepo(), txSvc)
	voucherSvc := voucher.NewService(db, vouchergorm.NewVoucherRepo(), txSvc)
	billingSvc := billing.NewService(billinggorm.NewBillingRuleRepo())
	orderSvc := order.NewService(db, ordergorm.NewOrderRepo(), accSvc, couponSvc, voucherSvc, billingSvc, txSvc)
	reconSvc := reconciliation.NewService(db, accSvc, txSvc)

	mux := http.NewServeMux()
	account.NewHandler(accSvc).Register(mux)
	coupon.NewHandler(couponSvc).Register(mux)
	voucher.NewHandler(voucherSvc).Register(mux)
	order.NewHandler(orderSvc).Register(mux)
	reconciliation.NewHandler(reconSvc).Register(mux)

	// 支付渠道（可选挂载）
	if channelName != "" {
		p, err := NewProvider(channelName)
		if err != nil {
			return nil, fmt.Errorf("init provider: %w", err)
		}
		channel.RegisterRoutes(mux, p)
	}
	return mux, nil
}

// NewProvider 根据渠道名从环境变量加载配置并创建 Provider。
func NewProvider(channelName string) (channel.Provider, error) {
	switch channelName {
	case "wechat":
		return channel.NewWechatPay(&wechat.Config{
			AppID:     os.Getenv("WECHAT_APP_ID"),
			MchID:     os.Getenv("WECHAT_MCH_ID"),
			APIv3Key:  os.Getenv("WECHAT_API_V3_KEY"),
			MchKey:    os.Getenv("WECHAT_MCH_KEY"),
			MchCert:   os.Getenv("WECHAT_MCH_CERT"),
			NotifyURL: os.Getenv("WECHAT_NOTIFY_URL"),
		})
	case "alipay":
		return channel.NewAlipayPay(&alipay.Config{
			AppID:      os.Getenv("ALIPAY_APP_ID"),
			PrivateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
			PublicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
			NotifyURL:  os.Getenv("ALIPAY_NOTIFY_URL"),
			ReturnURL:  os.Getenv("ALIPAY_RETURN_URL"),
		})
	default:
		return nil, fmt.Errorf("unsupported channel %q, want wechat or alipay", channelName)
	}
}
