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

	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/alipay"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/wechat"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/middleware"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	channelFlag := flag.String("channel", "", "支付渠道：wechat 或 alipay")
	flag.Parse()

	p, err := newProvider(*channelFlag)
	if err != nil {
		log.Fatalf("init provider: %v", err)
	}

	srv := channel.NewServer(*addr, p)
	srv.SetHandler(middleware.Logging(srv.Handler()))

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

	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
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
