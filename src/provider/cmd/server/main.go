// Package main 服务端入口：加载配置、组装依赖、启动服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	"github.com/quanttide/qtcloud-pay/src/provider/internal/middleware"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	ordergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/order/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/reconciliation"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	channelName := flag.String("channel", "", "支付渠道：wechat 或 alipay（可为空，仅启动账本 API）")
	flag.Parse()

	db, err := openDB()
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	mux := buildMux(db, *channelName)
	srv := &http.Server{Addr: *addr, Handler: middleware.Logging(mux)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("API server listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

// openDB 按配置（DB_DRIVER / DATABASE_URL）打开数据库并迁移全部模型。
// DB_DRIVER 为空或 sqlite 时使用开发库；postgres 时使用生产库。
func openDB() (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch os.Getenv("DB_DRIVER") {
	case "postgres":
		db, err = gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{TranslateError: true})
	default: // sqlite（开发默认）
		dsn := os.Getenv("DB_SQLITE_DSN")
		if dsn == "" {
			dsn = "qtcloud.db"
		}
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
	return db, nil
}

// buildMux 组装全部模块并返回路由。
func buildMux(db *gorm.DB, channelName string) *http.ServeMux {
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
		p, err := newProvider(channelName)
		if err != nil {
			log.Fatalf("init provider: %v", err)
		}
		channel.RegisterRoutes(mux, p)
	}
	return mux
}

// newProvider 根据渠道名从环境变量加载配置并创建 Provider。
func newProvider(channelName string) (channel.Provider, error) {
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
